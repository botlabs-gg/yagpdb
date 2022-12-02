// © 2016 Steve McCoy under the MIT license. See LICENSE for details.

package ogg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
)

// A Decoder decodes an ogg stream page-by-page with its Decode method.
type Decoder struct {
	r   io.Reader
	buf [maxPageSize]byte
}

// NewDecoder creates an ogg Decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// A Page represents a logical ogg page.
type Page struct {
	// Type is a bitmask of COP, BOS, and/or EOS.
	Type byte
	// Serial is the bitstream serial number.
	Serial uint32
	// Granule is the granule position, whose meaning is dependent on the encapsulated codec.
	Granule int64

	Page  uint32 // 18-21, sequence number of page in packet
	Crc   uint32 // 22-25
	Nsegs byte   // 26

	SegTbl []byte

	// Data is the raw packet data.
	// If Type & COP != 0, this is a continuation of the previous page's packet.
	Data []byte

	currentIndex int
	currentPos   int
}

// ReadPacket reads the next packet or returns io.EOF if no more.
// Complete is false when the the packet was uncomplete (it was the last one and will continue next page)
func (p *Page) ReadPacket() (packet []byte, complete bool, err error) {
	packet = make([]byte, 0)
	complete = false

	if p.currentIndex >= len(p.SegTbl) {
		return nil, true, io.EOF
	}

	// Read each segment from the segment table until we either ran out or hit the packet boundry
	for p.currentIndex < len(p.SegTbl) {

		segment := p.SegTbl[p.currentIndex]

		packet = append(packet, p.Data[p.currentPos:p.currentPos+int(segment)]...)

		p.currentPos += int(segment)
		p.currentIndex++

		if segment < 255 {
			complete = true
			break
		}
	}

	return
}

// ErrBadSegs is the error used when trying to decode a page with a segment table size less than 1.
var ErrBadSegs = errors.New("invalid segment table size")

// ErrBadCrc is the error used when an ogg page's CRC field does not match the CRC calculated by the Decoder.
type ErrBadCrc struct {
	Found    uint32
	Expected uint32
}

func (bc ErrBadCrc) Error() string {
	return "invalid crc in packet: got " + strconv.FormatInt(int64(bc.Found), 16) +
		", expected " + strconv.FormatInt(int64(bc.Expected), 16)
}

var oggs = []byte{'O', 'g', 'g', 'S'}

// Decode reads from d's Reader to the next ogg page, then returns the decoded Page or an error.
// The error may be io.EOF if that's what the Reader returned.
//
// The buffer underlying the returned Page's Packet is owned by the Decoder.
// It may be overwritten by subsequent calls to Decode.
//
// It is safe to call Decode concurrently on distinct Decoders if their Readers are distinct.
// Otherwise, the behavior is undefined.
func (d *Decoder) Decode() (Page, error) {
	hbuf := d.buf[0:HeaderSize]
	b := 0
	for {
		_, err := io.ReadFull(d.r, hbuf[b:])
		if err != nil {
			return Page{}, err
		}

		i := bytes.Index(hbuf, oggs)
		if i == 0 {
			break
		}

		if i < 0 {
			const n = HeaderSize
			if hbuf[n-1] == 'O' {
				i = n - 1
			} else if hbuf[n-2] == 'O' && hbuf[n-1] == 'g' {
				i = n - 2
			} else if hbuf[n-3] == 'O' && hbuf[n-2] == 'g' && hbuf[n-1] == 'g' {
				i = n - 3
			}
		}

		if i > 0 {
			b = copy(hbuf, hbuf[i:])
		}
	}

	var h pageHeader
	_ = binary.Read(bytes.NewBuffer(hbuf), byteOrder, &h)

	if h.Nsegs < 1 {
		return Page{}, ErrBadSegs
	}

	nsegs := int(h.Nsegs)
	segtbl := d.buf[HeaderSize : HeaderSize+nsegs]
	_, err := io.ReadFull(d.r, segtbl)
	if err != nil {
		return Page{}, err
	}

	pageDataLen := 0
	for _, l := range segtbl {
		pageDataLen += int(l)
	}

	packet := d.buf[HeaderSize+nsegs : HeaderSize+nsegs+pageDataLen]
	_, err = io.ReadFull(d.r, packet)
	if err != nil {
		return Page{}, err
	}

	page := d.buf[0 : HeaderSize+nsegs+pageDataLen]
	// Clear out existing crc before calculating it
	page[22] = 0
	page[23] = 0
	page[24] = 0
	page[25] = 0
	crc := crc32(page)
	if crc != h.Crc {
		return Page{}, ErrBadCrc{h.Crc, crc}
	}
	return Page{
		Type:    h.HeaderType,
		Serial:  h.Serial,
		Granule: h.Granule,
		Data:    packet,
		Page:    h.Page,
		Nsegs:   h.Nsegs,
		Crc:     h.Crc,
		SegTbl:  segtbl,
	}, nil
}
