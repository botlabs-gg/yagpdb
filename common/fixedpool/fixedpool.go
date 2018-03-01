package fixedpool

import (
	"sync"
	"time"

	"github.com/mediocregopher/radix.v2/redis"
)

// Pool is a simple connection pool for redis Clients. It will create a small
// pool of initial connections, and if more connections are needed they will be
// created on demand. If a connection is Put back and the pool is full it will
// be closed.
type Pool struct {
	pool chan *redis.Client
	df   DialFunc

	initDoneCh chan bool // used for tests
	stopOnce   sync.Once
	stopCh     chan bool

	// The network/address that the pool is connecting to. These are going to be
	// whatever was passed into the New function. These should not be
	// changed after the pool is initialized
	Network, Addr string
}

// DialFunc is a function which can be passed into NewCustom
type DialFunc func(network, addr string) (*redis.Client, error)

// NewCustom is like New except you can specify a DialFunc which will be
// used when creating new connections for the pool. The common use-case is to do
// authentication for new connections.
func NewCustom(network, addr string, size int, df DialFunc) (*Pool, error) {
	p := Pool{
		Network:    network,
		Addr:       addr,
		pool:       make(chan *redis.Client, size),
		df:         df,
		initDoneCh: make(chan bool),
		stopCh:     make(chan bool),
	}

	if size < 1 {
		return &p, nil
	}

	// set up a go-routine which will periodically ping connections in the pool.
	// if the pool is idle every connection will be hit once every 10 seconds.
	// we do some weird defer/wait stuff to ensure this always gets started no
	// matter what happens with the rest of the initialization
	startTickCh := make(chan struct{})
	defer close(startTickCh)
	go func() {
		tick := time.NewTicker(10 * time.Second / time.Duration(size))
		defer tick.Stop()
		<-startTickCh
		for {
			select {
			case <-p.stopCh:
				close(p.stopCh)
				return
			case <-tick.C:
				p.Cmd("PING")
			}
		}
	}()

	mkConn := func() error {
		client, err := df(network, addr)
		if err == nil {
			p.pool <- client
		}
		return err
	}

	// make one connection to make sure the redis instance is actually there
	if err := mkConn(); err != nil {
		return &p, err
	}

	// make the rest of the connections in the background, if any fail it's fine
	go func() {
		for i := 0; i < size-1; i++ {
			mkConn()
		}
		close(p.initDoneCh)
	}()

	return &p, nil
}

// New creates a new Pool whose connections are all created using
// redis.Dial(network, addr). The size indicates the maximum number of idle
// connections to have waiting to be used at any given moment. If an error is
// encountered an empty (but still usable) pool is returned alongside that error
func New(network, addr string, size int) (*Pool, error) {
	return NewCustom(network, addr, size, redis.Dial)
}

// Get retrieves an available redis client. If there are none available it will
// create a new one on the fly
func (p *Pool) Get() (*redis.Client, error) {
	select {
	case conn := <-p.pool:
		return conn, nil
	case <-time.After(time.Second * 10):
		panic("Ran out of connections?")
	}
}

// Put returns a client back to the pool. If the pool is full the client is
// closed instead. If the client is already closed (due to connection failure or
// what-have-you) it will not be put back in the pool
func (p *Pool) Put(conn *redis.Client) {
	if conn.LastCritical == nil {
		select {
		case p.pool <- conn:
		default:
			conn.Close()
		}
	} else {
		go func() {
			for {
				client, err := p.df(p.Network, p.Addr)
				if err == nil {
					p.pool <- client
					return
				}
				time.Sleep(time.Second)
			}
		}()
	}
}

// Cmd automatically gets one client from the pool, executes the given command
// (returning its result), and puts the client back in the pool
func (p *Pool) Cmd(cmd string, args ...interface{}) *redis.Resp {
	c, err := p.Get()
	if err != nil {
		return redis.NewResp(err)
	}
	defer p.Put(c)

	return c.Cmd(cmd, args...)
}

// Empty removes and calls Close() on all the connections currently in the pool.
// Assuming there are no other connections waiting to be Put back this method
// effectively closes and cleans up the pool.
func (p *Pool) Empty() {
	p.stopOnce.Do(func() {
		p.stopCh <- true
		<-p.stopCh
	})
	var conn *redis.Client
	for {
		select {
		case conn = <-p.pool:
			conn.Close()
		default:
			return
		}
	}
}

// Avail returns the number of connections currently available to be gotten from
// the Pool using Get. If the number is zero then subsequent calls to Get will
// be creating new connections on the fly
func (p *Pool) Avail() int {
	return len(p.pool)
}
