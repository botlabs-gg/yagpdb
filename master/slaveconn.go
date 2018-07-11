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
	sc.Conn.ConnClosedHanlder = func() {
		mu.Lock()
		if mainSlave == sc {
			mainSlave = newSlave
			newSlave = nil
		}
		mu.Unlock()
	}
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
	// Full slave migration with shard rescaling not implemented yet
	// case EvtSoftStartComplete:
	// 	go mainSlave.Conn.Send(EvtShutdown, nil)

	case EvtShardMigrationStart:
		data := dataInterface.(*ShardMigrationStartData)
		if data.FromThisSlave {
			logrus.Println("Main slave is ready for migration, readying slave, numshards: ", data.NumShards)
			// The main slave is ready for migration, prepare the new slave
			data.FromThisSlave = false

			newSlave.Conn.Send(EvtShardMigrationStart, data)
		} else {
			logrus.Println("Both slaves are ready for migration, starting with shard 0")
			// Both slaves are ready, start the transfer
			mainSlave.Conn.Send(EvtStopShard, &StopShardData{Shard: 0})
		}

	case EvtStopShard:
		// The main slave stopped a shard, resume it on the new slave
		data := dataInterface.(*StopShardData)

		logrus.Printf("Shard %d stopped, sending resume on new slave... (%d, %s) ", data.Shard, data.Sequence, data.SessionID)

		newSlave.Conn.Send(EvtResume, &ResumeShardData{
			Shard:     data.Shard,
			SessionID: data.SessionID,
			Sequence:  data.Sequence,
		})

	case EvtResume:
		data := dataInterface.(*ResumeShardData)
		logrus.Printf("Shard %d resumed, Stopping next shard", data.Shard)

		data.Shard++
		mainSlave.Conn.Send(EvtStopShard, &StopShardData{
			Shard:     data.Shard,
			SessionID: data.SessionID,
			Sequence:  data.Sequence,
		})

	case EvtGuildState:
		newSlave.Conn.Send(EvtGuildState, dataInterface)

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
	mainSlave.Conn.Send(EvtShardMigrationStart, &ShardMigrationStartData{FromThisSlave: true})
}
