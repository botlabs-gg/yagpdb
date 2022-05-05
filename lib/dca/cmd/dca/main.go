package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/dca"
)

// All global variables used within the program
var (
	// Magic bytes to write at the start of a DCA file

	// 1 for mono, 2 for stereo
	Channels int

	// Must be one of 8000, 12000, 16000, 24000, or 48000.
	// Discord only uses 48000 currently.
	FrameRate int

	// Rates from 500 to 512000 bits per second are meaningful
	// Discord only uses 8000 to 128000 and default is 64000
	Bitrate int

	// Must be one of voip, audio, or lowdelay.
	// DCA defaults to audio which is ideal for music
	// Not sure what Discord uses here, probably voip
	Application string

	// if true, dca sends raw output without any magic bytes or json metadata
	RawOutput bool

	FrameDuration int // Duration in ms of each audio frame

	// Wether variable bitrate is used or not
	VBR bool

	Volume int // change audio volume (256=normal)

	Threads int // change number of threads to use, 0 for auto

	Comment string // Comment left in the metadata

	//OpusEncoder *gopus.Encoder

	InFile      string
	CoverFormat string = "jpeg"

	OutFile string = "pipe:1"

	Quiet bool // disable all stats output

	err error
)

// init configures and parses the command line arguments
func init() {

	flag.StringVar(&InFile, "i", "pipe:0", "infile")
	flag.IntVar(&Volume, "vol", 256, "change audio volume (256=normal)")
	flag.IntVar(&Channels, "ac", 2, "audio channels")
	flag.IntVar(&FrameRate, "ar", 48000, "audio sampling rate")
	flag.IntVar(&FrameDuration, "as", 20, "audio frame duration can be 20, 40, or 60 (ms)")
	flag.IntVar(&Bitrate, "ab", 128, "audio encoding bitrate in kb/s can be 8 - 128")
	flag.IntVar(&Threads, "threads", 0, "number of threads to use, 0 for auto")
	flag.BoolVar(&VBR, "vbr", true, "variable bitrate")
	flag.BoolVar(&RawOutput, "raw", false, "Raw opus output (no metadata or magic bytes)")
	flag.StringVar(&Application, "aa", "audio", "audio application can be voip, audio, or lowdelay")
	flag.StringVar(&CoverFormat, "cf", "jpeg", "format the cover art will be encoded with")
	flag.StringVar(&Comment, "com", "", "leave a comment in the metadata")
	flag.BoolVar(&Quiet, "quiet", false, "disable stats output to stderr")

	flag.Parse()
}

// very simple program that wraps ffmpeg and outputs raw opus data frames
// with a uint16 header for each frame with the frame length in bytes
func main() {

	//////////////////////////////////////////////////////////////////////////
	// BLOCK : Basic setup and validation
	//////////////////////////////////////////////////////////////////////////

	// If only one argument provided assume it's a filename.
	if len(os.Args) == 2 {
		InFile = os.Args[1]
	}

	// If reading from a file, verify it exists.
	if InFile != "pipe:0" {
		if _, err := os.Stat(InFile); os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "warning: infile does not exist as a file on this system, will still continue on incase this is something else that ffmpeg accepts")
		}
	}

	// If reading from pipe, make sure pipe is open
	if InFile == "pipe:0" {
		fi, err := os.Stdin.Stat()
		if err != nil {
			fmt.Println(err)
			return
		}

		if (fi.Mode() & os.ModeCharDevice) == 0 {
		} else {
			fmt.Fprintln(os.Stderr, "error: stdin is not a pipe.")
			flag.Usage()
			return
		}
	}

	if Bitrate < 1 || Bitrate > 512 {
		Bitrate = 64 // Set to Discord default
	}

	//////////////////////////////////////////////////////////////////////////
	// BLOCK : Start reader and writer workers
	//////////////////////////////////////////////////////////////////////////

	options := &dca.EncodeOptions{
		Volume:        Volume,
		Channels:      Channels,
		FrameRate:     FrameRate,
		FrameDuration: FrameDuration,
		Bitrate:       Bitrate,
		RawOutput:     RawOutput,
		Application:   dca.AudioApplication(Application),
		CoverFormat:   CoverFormat,
		VBR:           VBR,
		Comment:       Comment,
		Threads:       Threads,
	}

	var session *dca.EncodeSession
	var output = os.Stdout

	if InFile == "pipe:0" {
		session, err = dca.EncodeMem(os.Stdin, options)
	} else {
		session, err = dca.EncodeFile(InFile, options)
	}

	if err != nil {
		fmt.Fprint(os.Stderr, "Failed creating an encoding session: ", err)
		os.Exit(1)
	}

	if !Quiet {
		go statusPrinter(session)
	}

	_, err := io.Copy(output, session)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nError writing:", err)
		os.Exit(1)
	} else if !Quiet {
		fmt.Fprintf(os.Stderr, "\nFinished encoding\n")
		fmt.Fprint(os.Stderr, "ffmpeg output\n\n", session.FFMPEGMessages())
	}
}

func statusPrinter(session *dca.EncodeSession) {
	ticker := time.NewTicker(time.Millisecond * 500)
	for {
		<-ticker.C
		stats := session.Stats()
		fmt.Fprintf(os.Stderr, "Time: %10s, Bitrate: %6.1fkbits/s, Size: %6dkB, Speed: %7.1f\r", stats.Duration.String(), stats.Bitrate, stats.Size, stats.Speed)
		if !session.Running() {
			break
		}
	}
}
