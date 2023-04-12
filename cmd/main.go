package main

import (
	"bufio"
	"fmt"
	"github.com/fatih/color"
	"github.com/jkittell/toolbox"
	"github.com/nexidian/gocliselect"
	"net/url"
	"os"
	"ottscanner"
	"runtime"
	"strings"
)

func menu() {
	// 	logger := ottscanner.NewTestingLogger(false)

	menu := gocliselect.NewMenu("HLS and DASH scanner")

	menu.AddItem("Print ABR Streams", "streams")
	menu.AddItem("Print Segments", "segments")
	menu.AddItem("Scan", "scan")
	menu.AddItem("Download", "download")
	// TODO add to menu
	// menu.AddItem("Random", "random")
	// menu.AddItem("Emulate Playback", "emulator")

	choice := menu.Display()

	fmt.Printf("Choice: %s\n", choice)

	fmt.Print("Enter URL: ")
	reader := bufio.NewReader(os.Stdin)
	// ReadString will block until the delimiter is entered
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return
	}

	// remove the delimeter from the string
	input = strings.TrimSuffix(input, "\n")
	fmt.Println(input)

	u, err := url.ParseRequestURI(input)
	if err != nil {
		fmt.Printf("err=%+v url=%+v\n", err, u)
		os.Exit(1)
	}

	scanner, err := ottscanner.New(input, 10)
	if err != nil {
		fmt.Println("could not start the scanner... ", err)
	}

	switch choice {
	case "streams":
		streams, err := scanner.Streams()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			for _, s := range streams {
				fmt.Println(s.Name())
			}
			os.Exit(0)
		}
	case "segments":
		segments, err := scanner.Segments()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			for _, s := range segments {
				fmt.Println(s)
			}
			os.Exit(0)
		}
	case "scan":
		scan, err := scanner.Scan()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			for segment, ok := range scan {
				var v string
				if ok {
					v = "OK"
					// ()
					fmt.Println(segment.URL(), "...", color.GreenString(v))
				} else {
					v = "ERR"
					fmt.Println(segment.URL(), "...", color.RedString(v))
				}
			}
			os.Exit(0)
		}
	case "download":
		tempDir := toolbox.TempDir()
		err := scanner.Download(tempDir, int64(runtime.NumCPU()))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			fmt.Println("segments download to ", tempDir)
			os.Exit(0)
		}
	default:
		fmt.Println("unknown selection")
		os.Exit(1)
	}
}

func main() {
	menu()
}
