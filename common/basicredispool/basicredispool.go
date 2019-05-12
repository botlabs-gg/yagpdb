package basicredispool

import (
	"github.com/jonas747/retryableredis"
	"github.com/mediocregopher/radix"
)

type Pool struct {
	pool chan radix.Conn
	size int
}

func NewPool(size int, conf *retryableredis.DialConfig) (*Pool, error) {
	p := &Pool{
		pool: make(chan radix.Conn, size),
		size: size,
	}

	for i := 0; i < size; i++ {
		c, err := retryableredis.Dial(conf)
		if err != nil {
			return nil, err
		}

		p.pool <- c
	}

	return p, nil
}

func (p *Pool) get() radix.Conn {
	return <-p.pool
}

func (p *Pool) put(c radix.Conn) {
	p.pool <- c
}

func (p *Pool) Do(a radix.Action) error {
	c := p.get()
	defer p.put(c)

	return c.Do(a)
}

func (p *Pool) Close() error {
	for i := 0; i < p.size; i++ {
		c := p.get()
		c.Close()
	}

	return nil
}
