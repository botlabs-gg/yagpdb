package pubsub

import (
	"encoding/json"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

// PublishRatelimit publishes a new global ratelimit hit on discord
func PublishRatelimit(rl *discordgo.RateLimit) {
	logger.Printf("Got 429: %s, %d", rl.Bucket, rl.RetryAfter)

	reset := time.Now().Add(rl.RetryAfter * time.Millisecond)
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

// EvictCacheSet backported version of the dev one
func EvictCacheSet(slot string, key interface{}) {
	marshalledKey, err := json.Marshal(key)
	if err != nil {
		logger.WithError(err).Error("failed marshaling CacheSet key")
		return
	}

	err = Publish("evict_guild_cache", -1, &evictCacheSetData{
		Name: slot,
		Key:  marshalledKey,
	})
	if err != nil {
		logger.WithError(err).Error("failed publishing guild cache eviction")
	}
}

type evictCacheSetData struct {
	Name string          `json:"name"`
	Key  json.RawMessage `json:"key"`
}
