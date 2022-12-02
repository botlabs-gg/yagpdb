package dshardorchestrator

import (
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
)

// Conn represents a connection from either node to the orchestrator or the other way around
// it implements common logic across both sides
type Conn struct {
	logger  Logger
	netConn net.Conn
	sendmu  sync.Mutex

	ID atomic.Value

	// called on incoming messages
	MessageHandler func(*Message)

	// called when the connection is closed
	ConnClosedHanlder func()
}

// ConnFromNetCon wraos a Conn around a net.Conn
func ConnFromNetCon(conn net.Conn, logger Logger) *Conn {
	c := &Conn{
		netConn: conn,
		logger:  logger,
	}

	c.ID.Store("unknown-" + strconv.FormatInt(getNewID(), 10))
	return c
}

func (c *Conn) Close() {
	c.netConn.Close()
}

// Listen starts listening for events on the connection
func (c *Conn) Listen() {
	c.Log(LogInfo, nil, "started listening for events...")

	var err error
	defer func() {
		if err != nil {
			c.Log(LogError, err, "an error occured while handling a connection")
		}

		c.netConn.Close()
		c.Log(LogInfo, nil, "connection closed")

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
			c.Log(LogError, err, "failed reading event id")
			return
		}

		// Read the body length
		_, err = c.netConn.Read(lenBuf)
		if err != nil {
			c.Log(LogError, err, "failed reading event length")
			return
		}

		id := EventType(binary.LittleEndian.Uint32(idBuf))
		l := binary.LittleEndian.Uint32(lenBuf)

		c.Log(LogDebug, err, fmt.Sprintf("inc message evt: %s, payload lenght: %d", id.String(), l))

		body := make([]byte, int(l))
		if l > 0 {
			// Read the body, if there was one
			_, err = io.ReadFull(c.netConn, body)
			if err != nil {
				c.Log(LogError, err, "failed reading message body")
				return
			}
		}

		msg := &Message{
			EvtID: id,
		}

		if id < 100 {
			decoded, err := DecodePayload(id, body)
			if err != nil {
				c.Log(LogError, err, "failed decoding message payload")
			}
			msg.DecodedBody = decoded
		} else {
			msg.RawBody = body
		}

		c.MessageHandler(msg)
	}
}

// Send sends the specified message over the connection, marshaling the data using json
// this locks the writer
func (c *Conn) Send(evtID EventType, data interface{}) error {

	encoded, err := EncodeMessage(evtID, data)
	if err != nil {
		return errors.WithMessage(err, "EncodeEvent")
	}

	c.sendmu.Lock()
	defer c.sendmu.Unlock()

	c.Log(LogDebug, nil, fmt.Sprintf("sending evt %s, len: %d", evtID.String(), len(encoded)))
	return c.SendNoLock(encoded)
}

// Same as Send but logs the error (usefull for launching send in new goroutines)
func (c *Conn) SendLogErr(evtID EventType, data interface{}) {
	err := c.Send(evtID, data)
	if err != nil {
		c.Log(LogError, err, "failed sending message")
	}
}

// SendNoLock sends the specified message over the connection, marshaling the data using json
// This does no locking and the caller is responsible for making sure its not called in multiple goroutines at the same time
func (c *Conn) SendNoLock(data []byte) error {
	_, err := c.netConn.Write(data)
	return errors.WithMessage(err, "netConn.Write")
}

// GetID is a simpler helper for retrieving the connection id
func (c *Conn) GetID() string {
	return c.ID.Load().(string)
}

func (c *Conn) Log(level LogLevel, err error, msg string) {
	if err != nil {
		msg = msg + ": " + err.Error()
	}

	id := c.GetID()

	msg = id + ": " + msg

	if c.logger == nil {
		StdLogInstance.Log(level, msg)
	} else {
		c.logger.Log(level, msg)
	}
}
