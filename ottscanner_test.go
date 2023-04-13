package ottscanner

import (
	"github.com/google/uuid"
	"github.com/jkittell/toolbox"
	"os"
	"path"
	"reflect"
	"testing"
)

var maxConcurrency int64 = 10

var testurls = []string{
	"http://devimages.apple.com/iphone/samples/bipbop/bipbopall.m3u8",
	"http://devstreaming-cdn.apple.com/videos/streaming/examples/img_bipbop_adv_example_ts/master.m3u8",
	//"http://devstreaming-cdn.apple.com/videos/streaming/examples/img_bipbop_adv_example_fmp4/master.m3u8",
	//"http://devstreaming-cdn.apple.com/videos/streaming/examples/bipbop_adv_example_hevc/master.m3u8",
}

func TestNew(t *testing.T) {
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(url, scanner.url) {
			t.Fatalf("expected: %v, got: %v", url, scanner.url)
		}
	}
}

func TestScanner_Streams(t *testing.T) {
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		streams, err := scanner.Streams()
		if err != nil {
			t.FailNow()
		}

		if len(streams) == 0 {
			t.FailNow()
		}

		for _, stream := range streams {
			if stream.name == "" {
				t.FailNow()
			}
		}
	}
}

func TestScanner_Segments(t *testing.T) {
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		segments, err := scanner.Segments()
		if err != nil {
			t.FailNow()
		}
		if len(segments) == 0 {
			t.FailNow()
		}
	}
}

func TestScanner_Scan(t *testing.T) {
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		scans, err := scanner.Scan()
		if err != nil {
			t.FailNow()
		}
		if len(scans) == 0 {
			t.Fatal("no segments scanned")
		}

		for _, ok := range scans {
			if !ok {
				t.FailNow()
			}
		}
	}
}

func TestScanner_Download(t *testing.T) {
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		directory := toolbox.TempDir()
		err = scanner.Download(path.Join(directory, uuid.New().String()), maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		for streamName, files := range scanner.files {
			if len(files) == 0 {
				t.Fatal("no segment files", streamName)
			}
			for _, f := range files {
				if !toolbox.FileExists(f.File()) {
					t.Fatal(f)
				}
			}
		}
	}
}

func TestScanner_Files(t *testing.T) {
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		err = scanner.Download(os.TempDir(), maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		// TODO check number of files against scan
	}
}

func TestScanner_EmulatePlayback(t *testing.T) {
	t.Skip()
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		err = scanner.EmulatePlayback()
		if err != nil {
			t.FailNow()
		}
	}
}

func TestScanner_Random(t *testing.T) {
	t.Skip()
	for _, url := range testurls {
		scanner, err := New(url, maxConcurrency)
		if err != nil {
			t.FailNow()
		}

		err = scanner.Random()
		if err != nil {
			t.FailNow()
		}
	}
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			New(url, maxConcurrency)
		}
	}
}

func BenchmarkClient_Streams(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.Streams()
		}
	}
}

func BenchmarkClient_Segments(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.Segments()
		}
	}
}

func BenchmarkClient_Scan(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.Scan()
		}
	}
}

func BenchmarkClient_Download(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.Download(os.TempDir(), maxConcurrency)
		}
	}
}

func BenchmarkStream_Files(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.Download(os.TempDir(), maxConcurrency)
		}
	}
}

func BenchmarkClient_EmulatePlayback(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.EmulatePlayback()
		}
	}
}

func BenchmarkClient_Random(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, url := range testurls {
			scanner, _ := New(url, maxConcurrency)
			scanner.Random()
		}
	}
}
