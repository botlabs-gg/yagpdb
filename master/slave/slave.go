package slave

import (
	"github.com/jonas747/yagpdb/master"
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
	c.baseConn.Send(master.EvtSlaveHello, master.SlaveHelloData{Running: true})
	c.reconnecting = false
	c.mu.Unlock()

	return true
}

func (c *Conn) HandleMessage(m *master.Message) {
	switch m.EvtID {
	case master.EvtFullStart:
		c.bot.FullStart()
	case master.EvtSoftStart:
		c.bot.SoftStart()
	case master.EvtShutdown:
		c.bot.Shutdown()
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
