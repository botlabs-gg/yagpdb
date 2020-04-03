package bot

import (
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var metricsShardStatuses = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "bot_shards_status",
	Help: "Shard statuses",
}, []string{"status"})

var metricsTotalShards = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "bot_shards_total",
	Help: "Total number of shards on this node",
})

var metricsMembersTotal = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "bot_members_total",
	Help: "Total number of members on this node",
})

var metricsGuildsTotal = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "bot_guilds_total",
	Help: "Total number of guilds on this node",
})

var metricsGuildRegionsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "bot_guild_regions_total",
	Help: "Total number of guilds on this node and their regions",
}, []string{"region"})

func runUpdateMetrics() {
	ticker := time.NewTicker(time.Second * 10)
	var lastGuildsUpdate time.Time
	for {
		<-ticker.C
		runUpdateShardMetrics()

		if time.Since(lastGuildsUpdate) > time.Minute {
			// update guild stats less frequently because its a somewhat heavy operation
			runUpdateGuildTotalsMetrics()
			lastGuildsUpdate = time.Now()
		}
	}
}

func runUpdateShardMetrics() {
	processShards := ReadyTracker.GetProcessShards()

	statuses := map[string]int{
		"LOADING":      0,
		"READY":        0,
		"DISCONNECTED": 0,
	}

	for _, shardID := range processShards {
		shard := ShardManager.Sessions[shardID]

		strStatus := ""
		status := shard.GatewayManager.Status()
		switch status {
		case discordgo.GatewayStatusResuming, discordgo.GatewayStatusIdentifying:
			strStatus = "LOADING"
		case discordgo.GatewayStatusReady:
			strStatus = "READY"
		default:
			strStatus = "DISCONNECTED"
		}

		statuses[strStatus]++
	}

	for k, v := range statuses {
		metricsShardStatuses.With(prometheus.Labels{"status": k}).Set(float64(v))
	}

	metricsTotalShards.Set(float64(len(processShards)))
}

func runUpdateGuildTotalsMetrics() {
	guilds := State.GuildsSlice(true)

	totalMembers := 0
	result := make(map[string]int)

	for _, g := range guilds {
		totalMembers += metricsCountGuild(g, result)
	}

	for region, count := range result {
		metricsGuildRegionsTotal.With(prometheus.Labels{"region": region}).Set(float64(count))
	}

	metricsGuildsTotal.Set(float64(len(guilds)))
	metricsMembersTotal.Set(float64(totalMembers))

}

func metricsCountGuild(g *dstate.GuildState, regions map[string]int) int {
	g.RLock()
	defer g.RUnlock()

	regions[g.Guild.Region]++
	return g.Guild.MemberCount
}
