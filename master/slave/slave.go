package slave

import (
	"github.com/jonas747/yagpdb/master"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	processingGuildStates = new(int64)
)

// ConnectToMaster attempts to connect to master ,if it fails it will launch a reconnect loop and wait until the master appears
func ConnectToMaster(bot Bot, addr string) (*Conn, error) {
	conn := &Conn{
		bot:     bot,
		address: addr,
	}

	go conn.reconnectLoop(false)

	return conn, nil
}

// Conn is a wrapper around master.Conn, and represents a connection to the master
type Conn struct {
	baseConn *master.Conn
	mu       sync.Mutex

	numShards    int
	address      string
	bot          Bot
	reconnecting bool
	sendQueue    [][]byte
}

func (c *Conn) connect() error {
	netConn, err := net.Dial("tcp", c.address)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.baseConn = master.ConnFromNetCon(netConn)
	c.baseConn.MessageHandler = c.handleMessage
	c.baseConn.ConnClosedHanlder = c.onClosedConn
	go c.baseConn.Listen()

	c.mu.Unlock()

	return nil
}

func (c *Conn) onClosedConn() {
	go c.reconnectLoop(true)
}

func (c *Conn) reconnectLoop(running bool) {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}

	c.reconnecting = true
	c.mu.Unlock()

	go func() {
		for {
			if c.TryReconnect(running) {
				break
			}

			time.Sleep(time.Second * 5)
		}
	}()
}

func (c *Conn) TryReconnect(running bool) bool {
	err := c.connect()
	if err != nil {
		return false
	}

	encoded, err := master.EncodeEvent(master.EvtSlaveHello, master.SlaveHelloData{Running: running})
	if err != nil {
		panic("Failed encoding hello " + err.Error())
	}

	c.mu.Lock()
	c.baseConn.SendNoLock(encoded)
	for _, v := range c.sendQueue {
		c.baseConn.SendNoLock(v)
	}
	c.reconnecting = false
	c.mu.Unlock()

	return true
}

func (c *Conn) handleMessage(m *master.Message) {
	var dataInterface interface{}

	if m.EvtID != master.EvtGuildState {
		dataInterfaceF, ok := master.EvtDataMap[m.EvtID]
		if ok {
			dataInterface = dataInterfaceF()
			err := msgpack.Unmarshal(m.Body, dataInterface)
			if err != nil {
				logrus.WithError(err).Error("Failed decoding incoming event")
				return
			}
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

		go c.SendLogErr(master.EvtShardMigrationStart, data, true)

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
		go c.SendLogErr(master.EvtStopShard, data, true)

	case master.EvtResume:
		// Master is telling us to resume a shard
		data := dataInterface.(*master.ResumeShardData)

		go c.handleResume(data)
	case master.EvtGuildState:
		atomic.AddInt64(processingGuildStates, 1)
		go c.handleGuildState(m.Body)
	}
}

func (c *Conn) handleGuildState(body []byte) {
	defer func() {
		atomic.AddInt64(processingGuildStates, -1)
	}()

	var dest master.GuildStateData
	err := msgpack.Unmarshal(body, &dest)
	if err != nil {
		logrus.WithError(err).Error("Failed decoding guildstate")
	}

	c.bot.LoadGuildState(&dest)
}

func (c *Conn) handleResume(data *master.ResumeShardData) {
	logrus.Println("Got resume event")

	// Wait for remaining guild states to be loaded before we resume, since they're handled concurrently
	for atomic.LoadInt64(processingGuildStates) > 0 {
		runtime.Gosched()
	}

	c.bot.StartShard(data.Shard, data.SessionID, data.Sequence)
	time.Sleep(time.Second)
	c.SendLogErr(master.EvtResume, data, true)
}

// Send sends the message to the master, if the connection is closed it will queue the message if queueFailed is set
func (c *Conn) Send(evtID master.EventType, body interface{}, queueFailed bool) error {
	encoded, err := master.EncodeEvent(evtID, body)
	if err != nil {
		return err
	}

	c.mu.Lock()
	if c.reconnecting {
		if queueFailed {
			c.sendQueue = append(c.sendQueue, encoded)
		}
		c.mu.Unlock()
		return nil
	}

	err = c.baseConn.SendNoLock(encoded)

	c.mu.Unlock()

	return err
}

func (c *Conn) SendLogErr(evtID master.EventType, body interface{}, queueFailed bool) {
	err := c.Send(evtID, body, queueFailed)
	if err != nil {
		logrus.WithError(err).Error("[SLAVE] Failed sending message to master")
	}
}
