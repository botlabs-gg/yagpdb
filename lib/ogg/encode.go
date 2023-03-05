// © 2016 Steve McCoy under the MIT license. See LICENSE for details.

package ogg

import (
	"bytes"
	"encoding/binary"
	"io"
)

// An Encoder encodes raw bytes into an ogg stream.
type Encoder struct {
	serial uint32
	w      io.Writer
	buf    [maxPageSize]byte
}

// NewEncoder creates an ogg encoder with the given serial ID.
// Multiple Encoders can be used to encode multiplexed logical streams
// by giving them distinct IDs. Users must be sure to encode the streams
// as specified by the ogg RFC:
// When Grouping, all BOS pages must come before the data
// and EOS pages, with the order of BOS pages defined by the encapsulated encoding.
// When Chaining, the EOS page of the first stream must be immediately followed by
// the BOS of the second stream, and so on.
//
// For more details, see
// http://xiph.org/ogg/doc/rfc3533.txt and
// http://xiph.org/ogg/doc/framing.html
func NewEncoder(id uint32, w io.Writer) *Encoder {
	return &Encoder{serial: id, w: w}
}

// EncodeBOS writes a beginning-of-stream packet to the ogg stream,
// using the provided granule position.
// If the packet is larger than can fit in a page, it is split into multiple
// pages with the continuation-of-packet flag set.
func (w *Encoder) EncodeBOS(granule int64, packet []byte) error {
	return w.writePacket(BOS, granule, packet)
}

// Encode writes a data packet to the ogg stream,
// using the provided granule position.
// If the packet is larger than can fit in a page, it is split into multiple
// pages with the continuation-of-packet flag set.
func (w *Encoder) Encode(granule int64, packet []byte) error {
	return w.writePacket(0, granule, packet)
}

// EncodeEOS writes an end-of-stream packet to the ogg stream.
func (w *Encoder) EncodeEOS() error {
	return w.writePacket(EOS, 0, nil)
}

// Todo allow multiple packets
func (w *Encoder) writePacket(kind byte, granule int64, packet []byte) error {
	h := pageHeader{
		OggS:       [4]byte{'O', 'g', 'g', 'S'},
		HeaderType: kind,
		Serial:     w.serial,
		Granule:    granule,
	}

	if len(packet) > MaxPacketSize {
		panic("Packet > MaxPacketSize")
	}

	err := w.writePage(packet, &h)
	if err != nil {
		return err
	}

	// last := (len(packet) / MaxPacketSize) * MaxPacketSize
	// h.HeaderType |= COP
	// for s < last {
	// 	h.Page++
	// 	e = s + MaxPacketSize
	// 	page = packet[s:e]
	// 	err = w.writePage(page, &h)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	s = e
	// }

	// if s != len(packet) {
	// 	err = w.writePage(packet[s:], &h)
	// }
	return nil
}

func (w *Encoder) writePage(page []byte, h *pageHeader) error {

	h.Nsegs = byte(len(page) / 255)

	rem := byte(len(page) % 255)
	if rem > 0 || len(page) == 0 {
		h.Nsegs++
	}

	segtbl := make([]byte, h.Nsegs)
	for i := 0; i < len(segtbl); i++ {
		segtbl[i] = 255
	}

	if rem > 0 || len(page) == 0 {
		segtbl[len(segtbl)-1] = rem
	}

	hb := bytes.NewBuffer(w.buf[0:0:cap(w.buf)])
	_ = binary.Write(hb, byteOrder, h)

	hb.Write(segtbl)
	hb.Write(page)

	bb := hb.Bytes()
	crc := crc32(bb)
	_ = binary.Write(bytes.NewBuffer(bb[22:22:26]), byteOrder, crc)

	_, err := hb.WriteTo(w.w)
	return err
}
