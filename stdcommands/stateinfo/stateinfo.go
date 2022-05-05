package stateinfo

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	Cooldown:     2,
	CmdCategory:  commands.CategoryDebug,
	Name:         "stateinfo",
	Description:  "Responds with state debug info",
	HideFromHelp: true,
	RunFunc:      cmdFuncStateInfo,
}

func cmdFuncStateInfo(data *dcmd.Data) (interface{}, error) {
	totalGuilds := 0
	totalMembers := 0
	guildChannel := 0
	totalMessages := 0

	// state := bot.State
	// totalChannels := len(state.Channels)
	// totalGuilds = len(state.Guilds)
	// gCop := state.GuildsSlice(false)

	shards := bot.ReadyTracker.GetProcessShards()

	for _, shard := range shards {
		guilds := bot.State.GetShardGuilds(int64(shard))
		totalGuilds += len(guilds)

		for _, g := range guilds {
			guildChannel += len(g.Channels)
		}
	}

	// stats := bot.State.StateStats()

	embed := &discordgo.MessageEmbed{
		Title: "State size",
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Guilds", Value: fmt.Sprint(totalGuilds), Inline: true},
			{Name: "Members", Value: fmt.Sprintf("%d", totalMembers), Inline: true},
			{Name: "Messages", Value: fmt.Sprintf("%d", totalMessages), Inline: true},
			{Name: "Guild Channels", Value: fmt.Sprintf("%d", guildChannel), Inline: true},
			// {Name: "Total Channels", Value: fmt.Sprintf("%d", totalChannels), Inline: true},
			// {Name: "Cache Hits/Misses", Value: fmt.Sprintf("%d - %d", stats.CacheHits, stats.CacheMisses), Inline: true},
			// {Name: "Members evicted total", Value: fmt.Sprintf("%d", stats.MembersRemovedTotal), Inline: true},
			// {Name: "Cache evicted total", Value: fmt.Sprintf("%d", stats.UserCachceEvictedTotal), Inline: true},
			// {Name: "Messages removed total", Value: fmt.Sprintf("%d", stats.MessagesRemovedTotal), Inline: true},
		},
	}

	return embed, nil
}
