package master

import (
	"github.com/sirupsen/logrus"
	"net"
)

// Represents a connection from a master server to a slave
type SlaveConn struct {
	Conn Conn
}

func NewSlaveConn(netConn net.Conn) *SlaveConn {
	sc := &SlaveConn{
		Conn: &Conn{
			ID:   getNewID,
			conn: netConn,
		},
	}

	sc.Conn.MessageHandler = sc.HandleMessage
	return sc
}

func (s *SlaveConn) listen() {
	s.Conn.Listen()
}

func (s *SlaveConn) HandleMessage(evtID uint32, body []byte) {
	logrus.Println("Got event ", evtID, " blen: ", len(body))
}
