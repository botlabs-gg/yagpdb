package master

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net"
)

// Represents a connection from a master server to a slave
type SlaveConn struct {
	Conn *Conn
}

func NewSlaveConn(netConn net.Conn) *SlaveConn {
	sc := &SlaveConn{
		Conn: &Conn{
			ID:      getNewID(),
			netConn: netConn,
		},
	}

	sc.Conn.MessageHandler = sc.HandleMessage
	return sc
}

func (s *SlaveConn) listen() {
	s.Conn.Listen()
}

func (s *SlaveConn) HandleMessage(msg *Message) {
	logrus.Println("Got event ", msg.EvtID, " blen: ", len(msg.Body))

	dataInterfaceF, ok := EvtDataMap[msg.EvtID]
	var dataInterface interface{}
	if ok {
		dataInterface = dataInterfaceF()
		err := json.Unmarshal(msg.Body, dataInterface)
		if err != nil {
			logrus.WithError(err).Error("Failed decoding incoming event")
			return
		}
	}

	mu.Lock()
	defer mu.Unlock()

	switch msg.EvtID {
	case EvtSlaveHello:
		hello := dataInterface.(*SlaveHelloData)
		s.HandleHello(hello)
	case EvtSoftStartComplete:
		go mainSlave.Conn.Send(EvtShutdown, nil)
	case EvtShutdown:
		if s == mainSlave {
			mainSlave = newSlave
			newSlave = nil
			go mainSlave.Conn.Send(EvtFullStart, nil)
		}
	}
}

func (s *SlaveConn) HandleHello(hello *SlaveHelloData) {
	if hello.Running {
		// Already running, assume this is the main slave
		mainSlave = s
		return
	}

	if mainSlave == nil {
		// No slave was running, just signal this new slave to fully start
		mainSlave = s
		mainSlave.Conn.Send(EvtFullStart, nil)
		return
	}

	if newSlave != nil {
		// Already a slave transfer going on
		// TODO: Handle this properly
		logrus.Error("Already transfering to a new slave!")
		return
	}

	logrus.Info("Starting slave transfer")

	// Start transfer
	newSlave = s
	s.Conn.Send(EvtSoftStart, nil)
}
