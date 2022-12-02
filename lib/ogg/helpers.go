package ogg

import (
	"bytes"
)

// PacketDecoder wraps around a decoder to easily read indiv packets
type PacketDecoder struct {
	D *Decoder

	currentPage         Page
	currentSegmentIndex int
	currentDataPos      int
	packetBuf           bytes.Buffer
}

func NewPacketDecoder(d *Decoder) *PacketDecoder {
	return &PacketDecoder{
		D: d,
	}
}

func (p *PacketDecoder) Decode() (packet []byte, newPage Page, err error) {
	if p.currentPage.Data == nil {
		newPage, err = p.D.Decode()
		if err != nil {
			return
		}
		p.currentPage = newPage
	}

	for {

		// Read the next packet from the segment table
		for p.currentSegmentIndex < len(p.currentPage.SegTbl) {
			segmentSize := p.currentPage.SegTbl[p.currentSegmentIndex]

			// 0 size means its the end of the last packet
			if segmentSize != 0 {
				p.packetBuf.Write(p.currentPage.Data[p.currentDataPos : p.currentDataPos+int(segmentSize)])
			}

			p.currentDataPos += int(segmentSize)
			p.currentSegmentIndex++

			// Anything less than a full packet means its the end of a packet
			if segmentSize < 0xff {
				packet = make([]byte, p.packetBuf.Len())
				p.packetBuf.Read(packet)
				p.packetBuf.Reset()
				return
			}
		}

		// If we got here then the packet continues in the next page
		// so grab the next page
		newPage, err = p.D.Decode()
		if err != nil {
			return
		}

		p.currentPage = newPage
		p.currentDataPos = 0
		p.currentSegmentIndex = 0
	}
}
