package pubsub

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cacheset"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// PublishRatelimit publishes a new global ratelimit hit on discord
func PublishRatelimit(rl *discordgo.RateLimit) {
	logger.Printf("Got 429: %s, %s", rl.Bucket, rl.RetryAfterDur())

	reset := time.Now().Add(rl.RetryAfterDur())
	err := Publish("global_ratelimit", -1, &globalRatelimitTriggeredEventData{
		Bucket: rl.Bucket,
		Reset:  reset,
	})
	if err != nil {
		logger.WithError(err).Error("failed publishing global ratelimit")
	}
}

type globalRatelimitTriggeredEventData struct {
	Reset  time.Time `json:"reset"`
	Bucket string    `json:"bucket"`
}

func handleGlobalRatelimtPusub(evt *Event) {
	data := evt.Data.(*globalRatelimitTriggeredEventData)
	common.BotSession.Ratelimiter.SetGlobalTriggered(data.Reset)
}

func handleEvictCoreConfigCache(evt *Event) {
	common.CoreServerConfigCache.Delete(int(evt.TargetGuildInt))
}

type evictCacheSetData struct {
	Name string          `json:"name"`
	Key  json.RawMessage `json:"key"`
}

func handleEvictCacheSet(evt *Event) {
	cast := evt.Data.(*evictCacheSetData)
	if slot := common.CacheSet.FindSlot(cast.Name); slot != nil {
		t := slot.NewKey()
		err := json.Unmarshal(cast.Key, t)
		if err != nil {
			logger.WithError(err).Error("failed unmarshaling CacheSet key")
		}

		keyConv := reflect.Indirect(reflect.ValueOf(t)).Interface()
		slot.Delete(keyConv)
	}
}

// EvictCacheSet sends a pubsub to evict the key on slot on all nodes if guildID is set to -1, otherwise the bot worker for that guild is the only one that handles it
func EvictCacheSet(slot *cacheset.Slot, key interface{}) {
	slot.Delete(key)

	marshalledKey, err := json.Marshal(key)
	if err != nil {
		logger.WithError(err).Error("failed marshaling CacheSet key")
		return
	}

	err = Publish("evict_cache_set", -1, &evictCacheSetData{
		Name: slot.Name(),
		Key:  marshalledKey,
	})
	if err != nil {
		logger.WithError(err).Error("failed publishing guild cache eviction")
	}
}
