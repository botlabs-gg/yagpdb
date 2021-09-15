package dcallvoice

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/bot"
	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/stdcommands/util"
	"github.com/jonas747/dcmd/v4"
	"github.com/jonas747/discordgo/v2"
)

var Command = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "dcallvoice",
	Description:          "Disconnects from all the voice channels the bot is in",
	HideFromHelp:         true,
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {

		vcs := make([]*discordgo.VoiceState, 0)

		processShards := bot.ReadyTracker.GetProcessShards()
		for _, shard := range processShards {
			guilds := bot.State.GetShardGuilds(int64(shard))
			for _, g := range guilds {
				vc := g.GetVoiceState(common.BotUser.ID)
				if vc != nil {
					vcs = append(vcs, vc)
					go bot.ShardManager.SessionForGuild(g.ID).GatewayManager.ChannelVoiceLeave(g.ID)
				}
			}
		}

		return fmt.Sprintf("Leaving %d voice channels...", len(vcs)), nil
	}),
}
