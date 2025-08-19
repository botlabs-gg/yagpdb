package dca

import (
	"testing"
)

func TestEncode(t *testing.T) {
	session, err := EncodeFile("testaudio.ogg", StdEncodeOptions)
	if err != nil {
		t.Fatal("Failed creating encoding session", err)
	}

	numFrames := 0
	for {
		_, err := session.ReadFrame()
		if err != nil {
			break
		}
		numFrames++
	}

	// Predetermined, probably gonna change the testing method somehow
	if numFrames != 755 {
		t.Errorf("Incorrect number of frames (got %d expected %d)", numFrames, 755)
		t.Fail()
	}
}
