package dca

import (
	"io"
	"os"
	"testing"
)

func TestDecode(t *testing.T) {
	file, err := os.Open("testaudio.dca")
	if err != nil {
		t.Error(err)
	}

	decoder := NewDecoder(file)

	err = decoder.ReadMetadata()
	if err != nil {
		t.Error(err)
	}

	frameCounter := 0
	for {
		_, err := decoder.OpusFrame()
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}
		frameCounter++
	}

	if frameCounter != 755 {
		t.Error("Incorrect number of frames")
	}
}
