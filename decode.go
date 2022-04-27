package dca

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"
)

var (
	ErrNotDCA        = errors.New("DCA Magic header not found, either not dca or raw dca frames")
	ErrNotFirstFrame = errors.New("Metadata can only be found in the first frame")
)

type Decoder struct {
	r *bufio.Reader

	Metadata      *Metadata
	FormatVersion int

	// Set to true after the first frame has been read
	firstFrameProcessed bool
}

// NewDecoder returns a new dca decoder
func NewDecoder(r io.Reader) *Decoder {
	decoder := &Decoder{
		r: bufio.NewReader(r),
	}

	return decoder
}

// ReadMetadata reads the first metadata frame
// OpusFrame will call this automatically if
func (d *Decoder) ReadMetadata() error {
	if d.firstFrameProcessed {
		return ErrNotFirstFrame
	}
	d.firstFrameProcessed = true

	fingerprint, err := d.r.Peek(4)
	if err != nil {
		return err
	}

	if string(fingerprint[:3]) != "DCA" {
		return ErrNotDCA
	}

	// We just peeked earlier, mark this portion as read
	d.r.Discard(4)

	// Read the format version
	version, err := strconv.ParseInt(string(fingerprint[3:]), 10, 32)
	if err != nil {
		return err
	}
	d.FormatVersion = int(version)

	// The length of the metadata
	var metaLen int32
	err = binary.Read(d.r, binary.LittleEndian, &metaLen)
	if err != nil {
		return err
	}

	// Read in the metadata itself
	jsonBuf := make([]byte, metaLen)
	err = binary.Read(d.r, binary.LittleEndian, &jsonBuf)
	if err != nil {
		return err
	}

	// And unmarshal it
	var metadata *Metadata
	err = json.Unmarshal(jsonBuf, &metadata)
	d.Metadata = metadata
	return err
}

// OpusFrame returns the next audio frame
// If this is the first frame it will also check for metadata in it
func (d *Decoder) OpusFrame() (frame []byte, err error) {
	if !d.firstFrameProcessed {
		// Check to see if this contains metadata and read the metadata if so
		magic, err := d.r.Peek(3)
		if err != nil {
			return nil, err
		}

		if string(magic) == "DCA" {
			err = d.ReadMetadata()
			if err != nil {
				return nil, err
			}
		}
	}

	frame, err = DecodeFrame(d.r)
	return
}

// FrameDuration implements OpusReader, returnining the specified duration per frame
func (d *Decoder) FrameDuration() time.Duration {
	if d.Metadata == nil {
		return 20
	}

	// I don't understand nick, why does it have to be like this nick, please nick, im not having a good time nick.
	// 960B = pcm framesize of 20ms 1 channel audio
	return time.Duration(((d.Metadata.Opus.FrameSize/d.Metadata.Opus.Channels)/960)*20) * time.Millisecond
}
