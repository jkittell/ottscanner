package ottscanner

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/jkittell/toolbox"
	"regexp"
	"strconv"
	"strings"
)

// decodeVariant returns a map where the keys are Segment urls and the values are byte ranges. If no bytes range then empty value for
// the key.
func decodeVariant(url string) (Segments, error) {
	logger.Debugf("decoding hls variant %s", url)
	// slice to return that gives full url
	var segments Segments

	playlist, err := toolbox.SendRequest(toolbox.GET, url, "", nil)
	if err != nil {
		return segments, newScannerError(err, fmt.Sprintf("unable to download hls variant playlist url %s", url))
	}

	// store byte range then continue to next line for Segment
	var byteRangeStart int
	var byteRangeSize int
	segmentFormats := []string{".ts", ".fmp4", ".cmfv", ".cmfa", ".aac", ".ac3", ".ec3", ".webvtt"}
	scanner := bufio.NewScanner(bytes.NewReader(playlist))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "#EXT-X-BYTERANGE") {
			// #EXT-X-BYTERANGE:44744@2304880
			// -H "Range: bytes=0-1023"
			// parse byte range here
			byteRangeValues := strings.Split(line, ":")
			if len(byteRangeValues) != 2 {
				message := fmt.Sprintf("problem parsing byte range: %s", byteRangeValues)
				return segments, newScannerError(errors.New(message), url)
			}
			byteRange := strings.Split(byteRangeValues[1], "@")
			startNumber, err := strconv.Atoi(byteRange[1])
			if err != nil {
				message := fmt.Sprintf("problem parsing byte range: %s", byteRangeValues)
				return segments, newScannerError(errors.New(message), url)
			}
			sizeNumber, err := strconv.Atoi(byteRange[0])
			if err != nil {
				message := fmt.Sprintf("problem parsing byte range: %s", byteRangeValues)
				return segments, newScannerError(errors.New(message), url)
			}
			byteRangeStart = startNumber
			byteRangeSize = sizeNumber
			continue
		} else {
			byteRangeStart = -1
			byteRangeSize = -1
		}

		for _, format := range segmentFormats {
			var match bool
			if strings.Contains(line, format) {
				match = true
				if match {
					if strings.Contains(line, "#EXT-X-MAP:URI=") {
						re := regexp.MustCompile(`"[^"]+"`)
						initSegment := re.FindString(line)
						if initSegment != "" {
							SegmentName := strings.Trim(initSegment, "\"")
							var SegmentURL string
							if !strings.Contains(SegmentName, "http") {
								baseURL := toolbox.BaseURL(url)
								SegmentURL = fmt.Sprintf("%s/%s", baseURL, SegmentName)
							} else {
								SegmentURL = SegmentName
							}
							seg := Segment{
								name:           SegmentName,
								url:            SegmentURL,
								byteRangeStart: byteRangeStart,
								byteRangeSize:  byteRangeSize,
							}
							segments = append(segments, seg)
						} else {
							return segments, newScannerError(errors.New("unable to parse init Segment: %s"), line)
						}
					} else {
						SegmentName := line
						var SegmentURL string
						if !strings.Contains(SegmentName, "http") {
							baseURL := toolbox.BaseURL(url)
							SegmentURL = fmt.Sprintf("%s/%s", baseURL, SegmentName)
						} else {
							SegmentURL = SegmentName
						}
						seg := Segment{
							name:           SegmentName,
							url:            SegmentURL,
							byteRangeStart: byteRangeStart,
							byteRangeSize:  byteRangeSize,
						}
						segments = append(segments, seg)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return segments, newScannerError(err, fmt.Sprintf("unable to parse hls variant playlist: %s", url))
	}

	return segments, nil
}

func decodeMaster(url string) (Streams, error) {
	logger.Debugf("decoding hls master playlist %s", url)
	var variants []string
	var Streams Streams
	playlist, err := toolbox.SendRequest(toolbox.GET, url, "", nil)
	if err != nil {
		return Streams, newScannerError(err, fmt.Sprintf("unable to download hls master playlist url %s", url))
	}

	scanner := bufio.NewScanner(bytes.NewReader(playlist))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "#EXT") && strings.Contains(line, "m3u8") {
			variants = append(variants, line)
		} else if strings.Contains(line, "#EXT-X-I-FRAME-STREAM-INF") || strings.Contains(line, "#EXT-X-MEDIA") {
			regEx := regexp.MustCompile("URI=\"(.*?)\"")
			match := regEx.MatchString(line)
			if match {
				s1 := regEx.FindString(line)
				_, s2, _ := strings.Cut(s1, "=")
				s3 := strings.Trim(s2, "\"")
				URI := s3
				logger.Debugf("variant found in master playlist %s", URI)
				variants = append(variants, URI)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Streams, newScannerError(err, fmt.Sprintf("unable to parse hls master playlist: %s", url))
	}

	for _, variant := range variants {
		var StreamURL string
		if !strings.Contains(variant, "http") {
			baseURL := toolbox.BaseURL(url)
			StreamURL = fmt.Sprintf("%s/%s", baseURL, variant)
		} else {
			StreamURL = variant
		}
		Stream := Stream{
			name:              variant,
			url:               StreamURL,
			masterPlaylistURL: url,
			segments:          nil,
		}
		Streams = append(Streams, Stream)
	}

	if len(Streams) > 0 {
		return Streams, nil
	} else {
		return Streams, newScannerError(err, fmt.Sprintf("no variant Streams found in hls master playlist: %s", url))
	}
}

func parseHLS(url string) (Streams, error) {
	var variants Streams
	variants, err := decodeMaster(url)
	if err != nil {
		return variants, newScannerError(err, "unable to parse hls master playlist")
	}

	if len(variants) > 0 {
		for _, v := range variants {
			segments, err := decodeVariant(v.url)
			if err != nil {
				return variants, err
			}
			v.segments = segments
		}
	} else {
		segments, err := decodeVariant(url)
		if err != nil {
			return variants, err
		}
		variants = Streams{
			Stream{
				name:              "",
				url:               url,
				masterPlaylistURL: url,
				segments:          segments,
			},
		}
	}

	return variants, nil
}
