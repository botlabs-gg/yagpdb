package master

import (
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
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
	var dataInterface interface{}

	if msg.EvtID != EvtGuildState {
		logrus.Println("Got event ", msg.EvtID.String(), " blen: ", len(msg.Body))

		dataInterfaceF, ok := EvtDataMap[msg.EvtID]
		if ok {
			dataInterface = dataInterfaceF()
			err := msgpack.Unmarshal(msg.Body, dataInterface)
			if err != nil {
				logrus.WithError(err).Error("Failed decoding incoming event")
				return
			}
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

			go newSlave.Conn.SendLogErr(EvtShardMigrationStart, data)
		} else {
			logrus.Println("Both slaves are ready for migration, starting with shard 0")
			// Both slaves are ready, start the transfer
			go mainSlave.Conn.SendLogErr(EvtStopShard, &StopShardData{Shard: 0})
		}

	case EvtStopShard:
		// The main slave stopped a shard, resume it on the new slave
		data := dataInterface.(*StopShardData)

		logrus.Printf("Shard %d stopped, sending resume on new slave... (%d, %s) ", data.Shard, data.Sequence, data.SessionID)

		go newSlave.Conn.SendLogErr(EvtResume, &ResumeShardData{
			Shard:     data.Shard,
			SessionID: data.SessionID,
			Sequence:  data.Sequence,
		})

	case EvtResume:
		data := dataInterface.(*ResumeShardData)
		logrus.Printf("Shard %d resumed, Stopping next shard", data.Shard)

		data.Shard++
		go mainSlave.Conn.SendLogErr(EvtStopShard, &StopShardData{
			Shard:     data.Shard,
			SessionID: data.SessionID,
			Sequence:  data.Sequence,
		})

	case EvtGuildState:
		newSlave.Conn.Send(EvtGuildState, msg.Body)
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
		go mainSlave.Conn.SendLogErr(EvtFullStart, nil)
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
