package cache

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix.v2/redis"
)

var (
	Cache = ccache.New(ccache.Configure())
)

func SetupCache() {
	pubsub.AddHandler("data_changed", handleChanged, "")
}

func handleChanged(event *pubsub.Event) {
	key, ok := event.Data.(*string)
	if !ok {
		logrus.Error("Invalid data_changed event", key)
		return
	}

	Cache.Delete(*key)
}

// Emits the data changed event on pubsub
func EmitChangedEvent(client *redis.Client, key string) error {
	if client == nil {
		var err error
		client, err = common.RedisPool.Get()
		if err != nil {
			return err
		}
		defer common.RedisPool.Put(client)
	}

	Cache.Delete(key)

	err := pubsub.Publish(client, "data_changed", "*", key)
	return err
}
