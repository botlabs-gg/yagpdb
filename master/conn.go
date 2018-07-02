package master

import (
	"encoding/binary"
	"github.com/sirupsen/logrus"
	"net"
)

type Conn struct {
	conn   net.Conn
	sendmu sync.Mutex
	ID     int64

	MessageHandler func(uint32, []byte)
}

func connFromNetCon(conn net.Conn) *Client {
	return &Client{
		conn: conn,
		ID:   getNewID(),
	}
}

func (c *Conn) Listen() {
	logrus.Info("Received new client ", c.id)

	var err error
	defer func() {
		if err != nil {
			logrus.WithError(err).Error("An error occured while handling a slave connection")
		}
		c.conn.Close()
	}()

	idBuf := make([]byte, 4)
	lenBuf := make([]byte, 4)
	for {

		_, err = c.conn.Read(idBuf)
		if err != nil {
			logrus.WithError(err).Error("Failed reading event id")
			return
		}

		_, err = c.conn.Read(lenBuf)
		if err != nil {
			logrus.WithError(err).Error("Failed reading event length")
			return
		}

		id := binary.LittleEndian.Uint32(idBuf)
		l := binary.LittleEndian.Uint32(lenBuf)
		body := make([]byte, int(l))
		if l > 0 {
			_, err = c.conn.Read(body)
			if err != nil {
				logrus.WithError(err).Error("Failed reading body")
				return
			}
		}

		c.MessageHandler(id, body)
	}
}
