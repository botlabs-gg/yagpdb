package mqueue

import (
	"sync"
	"time"

	"github.com/jonas747/yagpdb/common"
)

type Producer struct {
	backend Storage
}

// QueueMessage queues a message in the message queue
func (p *Producer) QueueMessage(elem *QueuedElement) error {
	elem.CreatedAt = time.Now()

	nextID, err := p.backend.NextID()
	if err != nil {
		return err
	}

	elem.ID = nextID
	return p.backend.AppendItem(elem)
}

var (
	producerOnce     sync.Once
	standardProducer *Producer
)

// QueueMessage queues a message in the message queue
func QueueMessage(elem *QueuedElement) error {
	producerOnce.Do(func() {
		standardProducer = &Producer{
			backend: &RedisBackend{
				pool: common.RedisPool,
			},
		}
	})

	return standardProducer.QueueMessage(elem)
}
