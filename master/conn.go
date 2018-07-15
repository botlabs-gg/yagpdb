package master

import (
	"bytes"
	"encoding/binary"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
	"io"
	"net"
	"sync"
)

// Simple helper to manage the underlying connection using locks
type Conn struct {
	netConn net.Conn
	sendmu  sync.Mutex
	ID      int64

	// Called on incoming messages
	MessageHandler func(*Message)

	// called when the connection is closed
	ConnClosedHanlder func()
}

// ConnFromNetCon wraos a Conn around a net.Conn
func ConnFromNetCon(conn net.Conn) *Conn {
	return &Conn{
		netConn: conn,
		ID:      getNewID(),
	}
}

// Listen starts listening for events on the connection
func (c *Conn) Listen() {
	logrus.Info("Master/Slave connection: starting listening for events ", c.ID)

	var err error
	defer func() {
		if err != nil {
			logrus.WithError(err).Error("An error occured while handling a connection")
		}
		c.netConn.Close()

		if c.ConnClosedHanlder != nil {
			c.ConnClosedHanlder()
		}
	}()

	idBuf := make([]byte, 4)
	lenBuf := make([]byte, 4)
	for {

		// Read the event id
		_, err = c.netConn.Read(idBuf)
		if err != nil {
			logrus.WithError(err).Error("Failed reading event id")
			return
		}

		// Read the body length
		_, err = c.netConn.Read(lenBuf)
		if err != nil {
			logrus.WithError(err).Error("Failed reading event length")
			return
		}

		id := binary.LittleEndian.Uint32(idBuf)
		l := binary.LittleEndian.Uint32(lenBuf)
		body := make([]byte, int(l))
		if l > 0 {
			// Read the body, if there was one
			_, err = io.ReadFull(c.netConn, body)
			if err != nil {
				logrus.WithError(err).Error("Failed reading body")
				return
			}
		}

		c.MessageHandler(&Message{EvtID: EventType(id), Body: body})
	}
}

// Send sends the specified message over the connection, marshaling the data using json
// this locks the writer
func (c *Conn) Send(evtID EventType, data interface{}) error {
	encoded, err := EncodeEvent(evtID, data)
	if err != nil {
		return errors.WithMessage(err, "EncodeEvent")
	}

	c.sendmu.Lock()
	defer c.sendmu.Unlock()

	return c.SendNoLock(encoded)
}

// Same as Send but logs the error (usefull for launching send in new goroutines)
func (c *Conn) SendLogErr(evtID EventType, data interface{}) {
	err := c.Send(evtID, data)
	if err != nil {
		logrus.WithError(err).Error("[MASTER] Failed sending message to slave")
	}
}

// SendNoLock sends the specified message over the connection, marshaling the data using json
// This does no locking and the caller is responsible for making sure its not called in multiple goroutines at the same time
func (c *Conn) SendNoLock(data []byte) error {

	_, err := c.netConn.Write(data)
	return errors.WithMessage(err, "netConn.Write")
}

// EncodeEvent encodes the event to the wire format
// The wire format is pretty basic, first 4 bytes is a uin32 representing what type of event this is
// next 4 bytes is another uin32 which represents the length of the body
// next n bytes is the body itself, which can even be empty in some cases
func EncodeEvent(evtID EventType, data interface{}) ([]byte, error) {
	var buf bytes.Buffer

	tmpBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmpBuf, uint32(evtID))
	buf.Write(tmpBuf)

	l := uint32(0)
	if data != nil {
		var serialized []byte
		if byteSlice, ok := data.([]byte); ok {
			serialized = byteSlice
		} else {
			var err error
			serialized, err = msgpack.Marshal(data)
			if err != nil {
				return nil, errors.WithMessage(err, "msgpack.Marshal")
			}
		}

		l = uint32(len(serialized))

		binary.LittleEndian.PutUint32(tmpBuf, l)

		buf.Write(tmpBuf)

		buf.Write(serialized)
	} else {
		buf.Write(make([]byte, 4))
	}

	return buf.Bytes(), nil
}
