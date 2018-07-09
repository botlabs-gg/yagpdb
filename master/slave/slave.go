package slave

import (
	"encoding/json"
	"github.com/jonas747/yagpdb/master"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

func ConnectToMaster(bot Bot, addr string) (*Conn, error) {
	conn := &Conn{
		bot:     bot,
		address: addr,
	}

	err := conn.Connect()
	if err != nil {
		return nil, err
	}

	conn.baseConn.SendNoLock(master.EvtSlaveHello, master.SlaveHelloData{
		Running: false,
	})

	return conn, nil
}

type QueuedMessage struct {
	EvtID uint32
	Data  interface{}
}

type Conn struct {
	baseConn *master.Conn
	mu       sync.Mutex

	numShards    int
	address      string
	bot          Bot
	reconnecting bool
	sendQueue    []*QueuedMessage
}

func (c *Conn) Connect() error {
	netConn, err := net.Dial("tcp", c.address)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.baseConn = master.ConnFromNetCon(netConn)
	c.baseConn.MessageHandler = c.HandleMessage
	c.baseConn.ConnClosedHanlder = c.OnClosedConn
	go c.baseConn.Listen()

	c.mu.Unlock()

	return nil
}

func (c *Conn) OnClosedConn() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}

	c.reconnecting = true
	c.mu.Unlock()

	go func() {
		for {
			if c.TryReconnect() {
				break
			}

			time.Sleep(time.Second)
		}
	}()
}

func (c *Conn) TryReconnect() bool {
	err := c.Connect()
	if err != nil {
		return false
	}

	c.mu.Lock()
	c.baseConn.SendNoLock(master.EvtSlaveHello, master.SlaveHelloData{Running: true})
	for _, v := range c.sendQueue {
		c.baseConn.SendNoLock(v.EvtID, v.Data)
	}
	c.reconnecting = false
	c.mu.Unlock()

	return true
}

func (c *Conn) HandleMessage(m *master.Message) {

	dataInterfaceF, ok := master.EvtDataMap[m.EvtID]
	var dataInterface interface{}
	if ok {
		dataInterface = dataInterfaceF()
		err := json.Unmarshal(m.Body, dataInterface)
		if err != nil {
			logrus.WithError(err).Error("Failed decoding incoming event")
			return
		}
	}

	switch m.EvtID {
	case master.EvtFullStart:
		c.bot.FullStart()

	case master.EvtSoftStart:
		c.bot.SoftStart()

	case master.EvtShutdown:
		c.bot.Shutdown()

	case master.EvtShardMigrationStart:
		// Master is telling us to prepare for shard migration, either to this slave or from this slave
		logrus.Println("Got shard migration start event, starting shard migration")
		data := dataInterface.(*master.ShardMigrationStartData)
		if data.FromThisSlave {
			numShards := c.bot.StartShardTransferFrom()
			c.mu.Lock()
			c.numShards = numShards
			c.mu.Unlock()

			data.NumShards = numShards
		} else {
			c.bot.StartShardTransferTo(data.NumShards)
		}

		c.Send(master.EvtShardMigrationStart, data, true)

	case master.EvtStopShard:
		logrus.Println("Got stopshard event")
		data := dataInterface.(*master.StopShardData)

		// Master is telling us to stop a shard
		c.mu.Lock()
		if data.Shard >= c.numShards {
			logrus.Println("Shard migration is done!")
			c.mu.Unlock()
			c.bot.Shutdown()
			return
		}
		c.mu.Unlock()

		sessionID, sequence := c.bot.StopShard(data.Shard)
		data.SessionID = sessionID
		data.Sequence = sequence
		c.Send(master.EvtStopShard, data, true)

	case master.EvtResume:
		// Master is telling us to resume a shard
		logrus.Println("Got resume event")
		data := dataInterface.(*master.ResumeShardData)
		c.bot.StartShard(data.Shard, data.SessionID, data.Sequence)
		time.Sleep(time.Second)
		c.Send(master.EvtResume, data, true)

	case master.EvtGuildState:
		logrus.Println("Got guildstate event")
		data := dataInterface.(*master.GuildStateData)
		c.bot.LoadGuildState(data)
	}
}

// Send sends the message to the master, if the connection is closed it will queue the message if queueFailed is set
func (c *Conn) Send(evtID uint32, body interface{}, queueFailed bool) error {
	c.mu.Lock()
	if c.reconnecting {
		if queueFailed {
			c.sendQueue = append(c.sendQueue, &QueuedMessage{EvtID: evtID, Data: body})
		}
		c.mu.Unlock()
		return nil
	}

	err := c.baseConn.SendNoLock(evtID, body)

	c.mu.Unlock()

	return err
}
