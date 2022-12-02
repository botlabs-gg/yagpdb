package mqueue

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/botlabs-gg/yagpdb/v2/bot"
)

// type WebhookCacheKey struct {
// 	GuildID   int64 `json:"guild_id"`
// 	ChannelID int64 `json:"channel_id"`
// }

var webhookCache = common.CacheSet.RegisterSlot("mqueue_webhook", nil, int64(0))

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, func(evt *eventsystem.EventData) {
		if p.server != nil {
			p.server.refreshWork <- false
		}

	}, eventsystem.EventYagShardReady)
}

var _ bot.LateBotInitHandler = (*Plugin)(nil)

// LateBotInit implements bot.LateBotInitHandler
func (p *Plugin) LateBotInit() {
	redisBackend := &RedisBackend{
		pool: common.RedisPool,
	}

	server := NewServer(redisBackend, &DiscordProcessor{})
	redisPubsub := RedisPushServer{
		pushwork:    server.PushWork,
		fullRefresh: server.refreshWork,
	}
	go server.Run()
	go redisPubsub.run()
	p.server = server

	logger.Info("Started mqueue server")
}

// StopBot implements bot.BotStopperHandler
func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	if p.server != nil {
		p.server.Stop <- wg
	} else {
		wg.Done()
	}
}

var (
	metricsRatelimit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yagpdb_mqueue_ratelimits_total",
		Help: "Ratelimits hit on the webhook session",
	})

	metricsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yagpdb_mqueue_processed_total",
		Help: "Total mqueue elements processed",
	}, []string{"source"})
)

func handleWebhookSessionRatelimit(s *discordgo.Session, r *discordgo.RateLimit) {
	if !r.TooManyRequests.Global {
		return
	}

	metricsRatelimit.Inc()
}
