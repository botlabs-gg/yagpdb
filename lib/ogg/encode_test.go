// © 2016 Steve McCoy under the MIT license. See LICENSE for details.

package ogg

import (
	"bytes"
	"io"
	"testing"
)

func TestBasicEncodeBOS(t *testing.T) {
	var b bytes.Buffer
	e := NewEncoder(1, &b)

	err := e.EncodeBOS(2, []byte("hello"))
	if err != nil {
		t.Fatal("unexpected EncodeBOS error:", err)
	}

	bb := b.Bytes()
	expect := []byte{
		'O', 'g', 'g', 'S',
		0,
		BOS,
		2, 0, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 0,
		0, 0, 0, 0,
		0x7e, 0xdf, 0x2e, 0x1e, // crc
		1,
		5, // segment table
		'h', 'e', 'l', 'l', 'o',
	}

	if !bytes.Equal(bb, expect) {
		t.Fatalf("bytes != expected:\n%x\n%x", bb, expect)
	}
}

func TestBasicEncode(t *testing.T) {
	var b bytes.Buffer
	e := NewEncoder(1, &b)

	err := e.Encode(2, []byte("hello"))
	if err != nil {
		t.Fatal("unexpected EncodeBOS error:", err)
	}

	bb := b.Bytes()
	expect := []byte{
		'O', 'g', 'g', 'S',
		0,
		0,
		2, 0, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 0,
		0, 0, 0, 0,
		0xc8, 0x21, 0xcc, 0x1c, // crc
		1,
		5, // segment table
		'h', 'e', 'l', 'l', 'o',
	}

	if !bytes.Equal(bb, expect) {
		t.Fatalf("bytes != expected:\n%x\n%x", bb, expect)
	}
}

func TestBasicEncodeEOS(t *testing.T) {
	var b bytes.Buffer
	e := NewEncoder(1, &b)

	err := e.EncodeEOS()
	if err != nil {
		t.Fatal("unexpected EncodeEOS error:", err)
	}

	bb := b.Bytes()
	expect := []byte{
		'O', 'g', 'g', 'S',
		0,
		EOS,
		0, 0, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 0,
		0, 0, 0, 0,
		0xc9, 0x22, 0xe8, 0x34, // crc
		1,
		0, // segment table
	}

	if !bytes.Equal(bb, expect) {
		t.Fatalf("bytes != expected:\n%x\n%x", bb, expect)
	}
}

// func TestLongEncode(t *testing.T) {
// 	var b bytes.Buffer
// 	e := NewEncoder(1, &b)

// 	var junk bytes.Buffer
// 	for i := 0; i < maxPageSize*2; i++ {
// 		junk.WriteByte('x')
// 	}

// 	err := e.Encode(2, junk.Bytes())
// 	if err != nil {
// 		t.Fatal("unexpected Encode error:", err)
// 	}

// 	bb := b.Bytes()
// 	expect := []byte{
// 		'O', 'g', 'g', 'S',
// 		0,
// 		0,
// 		2, 0, 0, 0, 0, 0, 0, 0,
// 		1, 0, 0, 0,
// 		0, 0, 0, 0,
// 		0xee, 0xb2, 0x0b, 0xca, // crc
// 		255,
// 	}

// 	if !bytes.Equal(bb[:HeaderSize], expect) {
// 		t.Fatalf("bytes != expected:\n%x\n%x", bb[:HeaderSize], expect)
// 	}

// 	expect2 := []byte{
// 		'O', 'g', 'g', 'S',
// 		0,
// 		COP,
// 		2, 0, 0, 0, 0, 0, 0, 0,
// 		1, 0, 0, 0,
// 		1, 0, 0, 0,
// 		0x17, 0x0d, 0xe6, 0xe6, // crc
// 		255,
// 	}

// 	if !bytes.Equal(bb[maxPageSize:maxPageSize+HeaderSize], expect2) {
// 		t.Fatalf("bytes != expected:\n%x\n%x", bb[maxPageSize:maxPageSize+HeaderSize], expect2)
// 	}
// }

type limitedWriter struct {
	N int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.N <= int64(len(p)) {
		n := w.N
		w.N = 0
		return int(n), io.ErrClosedPipe
	}

	w.N -= int64(len(p))
	return len(p), nil
}

// func TestShortWrites(t *testing.T) {
// 	e := NewEncoder(1, &limitedWriter{N: 0})
// 	err := e.Encode(2, []byte("hello"))
// 	if err != io.ErrClosedPipe {
// 		t.Fatal("expected ErrClosedPipe, got:", err)
// 	}

// 	e = NewEncoder(1, &limitedWriter{N: maxPageSize + 1})
// 	var junk bytes.Buffer
// 	for i := 0; i < maxPageSize*2; i++ {
// 		junk.WriteByte('x')
// 	}
// 	err = e.Encode(2, junk.Bytes())
// 	if err != io.ErrClosedPipe {
// 		t.Fatal("expected ErrClosedPipe, got:", err)
// 	}
// }
