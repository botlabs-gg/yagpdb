package dshardorchestrator

import (
	"bytes"
	"testing"
)

func TestEncodeMessage(t *testing.T) {
	data := "hello"
	encoded, err := EncodeMessage(EvtStartShards, data)
	if err != nil {
		t.Fatal("an error occured encoding the mssage: ", err)
	}

	expectedOutput := []byte{
		10, 0, 0, 0, // uint32 event id
		6, 0, 0, 0, // uint32 payload length
		165, 104, 101, 108, 108, 111, // msgpack encoded payload
	}

	if !bytes.Equal(encoded, expectedOutput) {
		t.Fatal("incorrect output, got: ", encoded)
	}
}
