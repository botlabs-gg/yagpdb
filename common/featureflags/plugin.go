package featureflags

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
)

var logger = common.GetPluginLogger(&Plugin{})

// Plugin represents the mqueue plugin
type Plugin struct {
	stopBGWorker chan *sync.WaitGroup
}

// PluginInfo implements common.Plugin
func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "featureflags",
		SysName:  "featureflags",
		Category: common.PluginCategoryCore,
	}
}

type EvictCacheData struct {
	GuildID int64 `json:"guild_id"`
}

// RegisterPlugin registers the mqueue plugin into the plugin system and also initializes it
func RegisterPlugin() {
	p := &Plugin{
		stopBGWorker: make(chan *sync.WaitGroup),
	}

	common.RegisterPlugin(p)

	pubsub.AddHandler(evictCachePubSubEvent, handleInvalidateCacheFor, nil)
	pubsub.AddHandler(evictCachePubSubEvent2, handleInvalidateCacheFor2, EvictCacheData{})
}

// Invalidate the cache when the rules have changed
func handleInvalidateCacheFor(event *pubsub.Event) {
	EvictCacheForGuild(event.TargetGuildInt)
}

func handleInvalidateCacheFor2(event *pubsub.Event) {
	dataCast := event.Data.(*EvictCacheData)
	EvictCacheForGuild(dataCast.GuildID)
}

// EvictCacheForGuild evicts the specified guild's featureflag cache
func EvictCacheForGuild(guildID int64) {
	cacheID := (guildID >> 22) % int64(len(caches))
	caches[cacheID].invalidateGuild(guildID)
}
