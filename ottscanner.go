package ottscanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jkittell/toolbox"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/semaphore"
	"os"
	"path"
	"strings"
	"sync"
)

var logger = NewTestingLogger(true)

type ContentFormat byte

const (
	HLS ContentFormat = iota
	DASH
)

type scannerError struct {
	Context string
	Err     error
}

func (se *scannerError) Error() string {
	return fmt.Sprintf("%s: %v", se.Context, se.Err)
}

func newScannerError(err error, info string) *scannerError {
	return &scannerError{
		Context: info,
		Err:     err,
	}
}

type Segments []Segment

type Segment struct {
	name           string
	url            string
	byteRangeStart int
	byteRangeSize  int
}

// Streams are playlists for each bitrate
type Streams []Stream

type Stream struct {
	name              string
	url               string
	masterPlaylistURL string
	segments          Segments
}

type scanner struct {
	url            string
	format         ContentFormat
	streams        Streams
	files          map[string][]SegmentDownload
	maxConcurrency int64
}

func (s *Segment) URL() string {
	return s.url
}

func (s *Segment) Name() string {
	return s.name
}

func (s *Segment) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Segment) ToString() string {
	data, _ := s.ToJSON()
	return string(data)
}

func (s *Stream) ToString() string {
	data, _ := s.ToJSON()
	return string(data)
}

func (s *Stream) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Stream) Name() string {
	return s.name
}

// Files returns a map of stream name and the corresponding
// segment file locations for that steam.
func (s *scanner) Files() map[string][]SegmentDownload {
	return s.files
}

// addFile checks if the file is already in the slice of
// files for the streams and if not adds it to the slice of
// file locations for the stream.
func (s *scanner) addFile(streamName string, download SegmentDownload) {
	s.files[streamName] = append(s.files[streamName], download)
}

// parse collects the ABR streams and segments from the playlist/manifest
func (s *scanner) parse() (Streams, error) {
	var streams Streams
	if s.format == HLS {
		return parseHLS(s.url)
	} else if s.format == DASH {
		return parseDASH(s.url)
	} else {
		err := errors.New("unable to determine if hls or dash")
		return streams, newScannerError(err, "parsing playlist")
	}
}

// TODO
// Random downloads segments randomly
func (s *scanner) Random() error {
	// TODO
	return newScannerError(errors.New("not yet implemented"), "go away")
}

// TODO
// EmulatePlayback will select a stream then download segments
// in a buffer from the live edge.
func (s *scanner) EmulatePlayback() error {
	// TODO
	return newScannerError(errors.New("not yet implemented"), "go away")
}

func (sd *SegmentDownload) Error() error {
	return sd.err
}

func (sd *SegmentDownload) File() string {
	return sd.filePath
}

type SegmentDownload struct {
	filePath string
	err      error
}

func downloader(done chan bool, results map[string][]SegmentDownload, directory string, str Stream, maxConcurrency int64) {
	sem := semaphore.NewWeighted(maxConcurrency)
	ctx := context.TODO()
	var wg sync.WaitGroup
	var mutex sync.RWMutex

	numberOfSegments := len(str.segments)
	if numberOfSegments == 0 {
		panic(newScannerError(errors.New("no segments to download"), str.name))
	}
	bar := progressbar.New(numberOfSegments)
	logger.Debugf("\ndownloading %d segments for stream: %s\n", numberOfSegments, str.name)
	// loop through the segments decoded from the playlist
	for _, segment := range str.segments {
		if err := sem.Acquire(ctx, 1); err != nil {
			panic(newScannerError(err, "could not acquire semaphore while downloading segments"))
		}

		wg.Add(1)
		go func(segment Segment) {
			defer wg.Done()
			fileName := path.Base(segment.url)
			filePath := path.Join(directory, fileName)
			var err error
			if segment.byteRangeStart > -1 && segment.byteRangeSize > -1 {
				headers := make(map[string]string)
				// "Range: bytes=0-1023"
				byteRange := fmt.Sprintf("%d-%d", segment.byteRangeStart, segment.byteRangeStart+segment.byteRangeSize)
				headers["Range"] = byteRange
				_, err = toolbox.DownloadFile(filePath, segment.url, headers)
			} else {
				_, err = toolbox.DownloadFile(filePath, segment.url, nil)
			}

			var download SegmentDownload
			if err != nil {
				download.err = newScannerError(err, segment.ToString())
			} else {
				download.filePath = filePath
				mutex.Lock()
				results[str.name] = append(results[str.name], download)
				mutex.Unlock()
			}
			sem.Release(1)
			bar.Add(1)
		}(segment)
	}
	wg.Wait()
	done <- true
	fmt.Println()
}

func downloadDASHSegments(directory, manifestURL string, streams Streams, maxConcurrency int64) (map[string][]SegmentDownload, error) {
	results := make(map[string][]SegmentDownload)
	for _, stream := range streams {
		// create a directory for the stream segments to be downloaded into
		streamDirectory := path.Join(directory, stream.name)
		if err := os.MkdirAll(streamDirectory, os.ModePerm); err != nil {
			return results, newScannerError(err, fmt.Sprintf("error creating directory to download segments: %s", streamDirectory))
		}

		// download the manifest into the directory
		playlistFileName := path.Base(manifestURL)
		playlistPath := path.Join(streamDirectory, playlistFileName)
		_, err := toolbox.DownloadFile(playlistPath, manifestURL, nil)
		if err != nil {
			return results, newScannerError(err, fmt.Sprintf("error downloading playlist: %s", stream.url))
		}

		// if any segments found for the stream download them into the directory for their ABR stream
		if len(stream.segments) > 0 {
			done := make(chan bool, 1)
			downloader(done, results, streamDirectory, stream, maxConcurrency)
			<-done
		} else {
			return results, newScannerError(errors.New("no segments to download"), stream.ToString())
		}
	}

	return results, nil
}

func downloadHLSSegments(directory string, streams Streams, maxConcurrency int64) (map[string][]SegmentDownload, error) {
	results := make(map[string][]SegmentDownload)
	for _, stream := range streams {
		// create a directory for the stream segments to be downloaded into
		streamDirectory := path.Join(directory, stream.name)
		if err := os.MkdirAll(streamDirectory, os.ModePerm); err != nil {
			return results, newScannerError(err, fmt.Sprintf("error creating directory to download segments: %s", streamDirectory))
		}

		// download the ABR stream playlist into the directory
		playlistFileName := path.Base(stream.url)
		playlistPath := path.Join(streamDirectory, playlistFileName)
		_, err := toolbox.DownloadFile(playlistPath, stream.url, nil)
		if err != nil {
			return results, newScannerError(err, fmt.Sprintf("error downloading playlist: %s", stream.url))
		}

		// get the full url of each segment in the ABR stream
		segmentsDecoded, err := decodeVariant(stream.url)
		if err != nil {
			return results, newScannerError(err, fmt.Sprintf("error getting segment urls from playlist: %s", stream.url))
		}

		// adding segments to the slice of segments of the stream
		stream.segments = append(stream.segments, segmentsDecoded...)

		// if any segments found for the stream download them into the directory for their ABR stream
		if len(stream.segments) > 0 {
			done := make(chan bool, 1)
			downloader(done, results, streamDirectory, stream, maxConcurrency)
			<-done

		} else {
			return results, newScannerError(errors.New("no segments to download"), stream.name)
		}
	}

	return results, nil
}

// Download downloads the segments and stores the ABR stream names
// and their corresponding segments. Retrieve the stream names and the
// downloaded file locations by calling scanner.Files().
func (s *scanner) Download(directory string, maxConcurrency int64) error {
	fmt.Println("downloading segments for playlist: ", s.url)
	streams, err := s.Streams()
	if err != nil {
		return newScannerError(err, fmt.Sprintf("error getting streams for download: %s", s.url))
	}

	numberOfStreams := len(streams)

	if numberOfStreams > 0 {
		if s.format == HLS {
			results, err := downloadHLSSegments(directory, streams, maxConcurrency)
			if err != nil {
				return newScannerError(err, fmt.Sprintf("error downloading hls segments: %s", s.url))
			}
			s.files = results
		} else if s.format == DASH {
			results, err := downloadDASHSegments(directory, s.url, streams, maxConcurrency)
			if err != nil {
				return newScannerError(err, fmt.Sprintf("error downloading hls segments: %s", s.url))
			}
			s.files = results
		} else {
			return newScannerError(errors.New("unable to determine if this is hls or dash when downloading segments"), s.url)
		}
	} else {
		return newScannerError(errors.New("no ABR streams"), s.url)
	}
	return nil
}

// Scan will do a head request on each segment and verify 200 response code
// and return a map of the segment scanned and if it was scanned successfully.
func (s *scanner) Scan() (map[Segment]bool, error) {
	results := make(map[Segment]bool)
	segments, err := s.Segments()
	if err != nil || len(segments) == 0 {
		return results, newScannerError(err, fmt.Sprintf("error getting segments: %s", s.url))
	}

	var wg sync.WaitGroup
	var mutex sync.RWMutex
	sem := semaphore.NewWeighted(s.maxConcurrency)
	ctx := context.TODO()
	for _, segment := range segments {
		if err := sem.Acquire(ctx, 1); err != nil {
			return results, newScannerError(err, "could not acquire semaphore while scanning segments")
		}
		wg.Add(1)
		// you have to pass the segment variable into the goroutine
		go func(segment Segment) {
			defer wg.Done()
			headers := make(map[string]string)
			if segment.byteRangeStart > -1 && segment.byteRangeSize > -1 {
				// "Range: bytes=0-1023"
				byteRangeEnd := segment.byteRangeStart + segment.byteRangeSize
				headers["Range"] = fmt.Sprintf("%d-%d", segment.byteRangeStart, byteRangeEnd)
			}
			_, err := toolbox.SendRequest(toolbox.HEAD, segment.url, "", headers)
			if err != nil {
				mutex.Lock()
				results[segment] = false
				mutex.Unlock()
			} else {
				// if no error we increment counter
				mutex.Lock()
				results[segment] = true
				mutex.Unlock()
			}
			sem.Release(1)
		}(segment)
	}

	wg.Wait()
	return results, nil
}

// Segments returns a slice of segment urls
func (s *scanner) Segments() (Segments, error) {
	var segments Segments
	streams, err := s.Streams()
	if err != nil {
		return segments, newScannerError(err, fmt.Sprintf("error getting streams: %s", s.url))
	}
	for _, stream := range streams {
		if s.format == HLS {
			segmentsDecoded, err := decodeVariant(stream.url)
			if err != nil {
				return segments, newScannerError(err, fmt.Sprintf("error getting segments: %s", stream.url))
			}
			segments = append(segments, segmentsDecoded...)
		} else if s.format == DASH {
			dashSegments := stream.segments
			if err != nil {
				return segments, newScannerError(err, fmt.Sprintf("error getting segments: %s", stream.url))
			}
			segments = append(segments, dashSegments...)
		} else {
			return segments, newScannerError(errors.New("unable to determine if this is hls or dash when getting segments"), s.url)
		}

	}
	return segments, nil
}

// Streams returns a map of stream name and url
func (s *scanner) Streams() (Streams, error) {
	logger.Debugf("checking url: %s", s.url)
	_, err := toolbox.SendRequest(toolbox.HEAD, s.url, "", nil)
	if err != nil {
		return Streams{}, newScannerError(err, fmt.Sprintf("error checking playlist: %s", s.url))
	}
	switch s.format {
	case HLS:
		logger.Infof("getting streams for hls playlist: %s", s.url)
		streams, err := parseHLS(s.url)
		if err != nil {
			return streams, newScannerError(err, fmt.Sprintf("error getting abr streams for hls: %s", s.url))
		}
		return streams, nil
	case DASH:
		logger.Infof("getting streams for dash playlist: %s", s.url)
		streams, err := parseDASH(s.url)
		if err != nil {
			return streams, newScannerError(err, fmt.Sprintf("error getting abr streams for dash: %s", s.url))
		}
		return streams, nil
	default:
		var str Streams
		return str, newScannerError(errors.New("unknown format"), "is this hls or dash?")
	}
}

func New(url string, maxConcurrency int64) (*scanner, error) {
	var format ContentFormat
	if strings.Contains(url, ".m3u8") {
		format = HLS
	} else if strings.Contains(url, ".mpd") {
		format = DASH
	} else {
		fmt.Println()
		return nil, newScannerError(errors.New("cannot determine if this url is hls or dash"), url)
	}
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	scanner := &scanner{
		url:            url,
		format:         format,
		streams:        Streams{},
		maxConcurrency: maxConcurrency,
	}
	return scanner, nil
}
