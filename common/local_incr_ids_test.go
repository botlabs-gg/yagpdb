package common

import (
	"testing"
)

func TestPQIncrID(t *testing.T) {
	tkey := "key1"

	id1, err := GenLocalIncrIDPQ(nil, 0, tkey)
	if err != nil {
		t.Error(err)
		return
	}

	if id1 != 1 {
		t.Errorf("Expected 1 got %d", id1)
		return
	}

	// should be increased
	id2, err := GenLocalIncrIDPQ(nil, 0, tkey)
	if err != nil {
		t.Error(err)
		return
	}

	if id2 != 2 {
		t.Errorf("Expected 2, got %d", id2)
		return
	}

	// test another guild id
	id3, err := GenLocalIncrIDPQ(nil, 1, tkey)
	if err != nil {
		t.Error(err)
		return
	}

	if id3 != 1 {
		t.Errorf("Expected 1, got %d", id3)
		return
	}

	// test another key with same guild id
	id4, err := GenLocalIncrIDPQ(nil, 0, tkey+"different")
	if err != nil {
		t.Error(err)
		return
	}

	if id4 != 1 {
		t.Errorf("Expected 1, got %d", id4)
		return
	}
}
