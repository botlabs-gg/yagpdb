package mqueue

import (
	"encoding/json"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/mediocregopher/radix/v3"
)

var _ Storage = (*RedisBackend)(nil)

type RedisBackend struct {
	pool *radix.Pool
}

func NewRedisBackend(pool *radix.Pool) *RedisBackend {
	return &RedisBackend{
		pool: pool,
	}
}

func (rb *RedisBackend) GetFullQueue() ([]*workItem, error) {
	var results [][]byte

	err := rb.pool.Do(radix.Cmd(&results, "ZRANGEBYSCORE", "mqueue", "-1", "+inf"))
	if err != nil {
		logger.WithError(err).Error("Failed polling redis mqueue")
		return nil, err
	}

	totalWork := make([]*workItem, 0, len(results))

	for _, v := range results {
		var dec QueuedElement
		err = json.Unmarshal(v, &dec)
		if err != nil {
			logger.WithError(err).Error("Failed decoding queued mqueue element from full refresh")
		} else {
			totalWork = append(totalWork, &workItem{
				Elem: &dec,
				Raw:  v,
			})
		}
	}

	return totalWork, nil
}

const mqueue_pubsub_key = "mqueue_pubsub"

func (rb *RedisBackend) AppendItem(elem *QueuedElement) error {
	serialized, err := json.Marshal(elem)
	if err != nil {
		// logger.WithError(err).Error("Failed marshaling mqueue element")
		return err
	}

	err = rb.pool.Do(radix.Cmd(nil, "ZADD", "mqueue", "-1", string(serialized)))
	if err != nil {
		return err
	}

	err = rb.pool.Do(radix.Cmd(nil, "PUBLISH", mqueue_pubsub_key, string(serialized)))
	return err
}

func (rb *RedisBackend) DelItem(item *workItem) error {
	return rb.pool.Do(radix.Cmd(nil, "ZREM", "mqueue", string(item.Raw)))
}

func (rb *RedisBackend) NextID() (next int64, err error) {
	err = rb.pool.Do(radix.Cmd(&next, "INCR", "mqueue_id_counter"))
	return
}

type RedisPushServer struct {
	pushwork    chan *workItem
	fullRefresh chan bool
	selectDB    int
}

func (rp *RedisPushServer) run() {
	var opts []radix.PersistentPubSubOpt
	if rp.selectDB != 0 {
		opts = append(opts, radix.PersistentPubSubConnFunc(func(network string, addr string) (radix.Conn, error) {
			return radix.Dial(network, addr, radix.DialSelectDB(rp.selectDB))
		}))
	}

	conn, err := radix.PersistentPubSubWithOpts("tcp", common.RedisAddr, opts...)
	if err != nil {
		panic(err)
	}

	msgChan := make(chan radix.PubSubMessage, 100)
	if err := conn.Subscribe(msgChan, "mqueue_pubsub"); err != nil {
		panic(err)
	}

	rp.fullRefresh <- true

	for msg := range msgChan {
		if len(msg.Message) < 1 {
			continue
		}

		var dec *QueuedElement
		err = json.Unmarshal(msg.Message, &dec)
		if err != nil {
			logger.WithError(err).Error("failed decoding mqueue pubsub message")
			continue
		}

		rp.pushwork <- &workItem{
			Elem: dec,
			Raw:  msg.Message,
		}
	}
}
