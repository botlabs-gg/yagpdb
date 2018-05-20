package stateinfo

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "stateinfo",
	Description:          "Responds with state debug info",
	HideFromHelp:         true,
	RunFunc:              cmdFuncStateInfo,
}

func cmdFuncStateInfo(data *dcmd.Data) (interface{}, error) {
	totalGuilds := 0
	totalMembers := 0
	totalChannels := 0
	totalMessages := 0

	state := bot.State
	state.RLock()
	defer state.RUnlock()

	totalGuilds = len(state.Guilds)

	for _, g := range state.Guilds {

		state.RUnlock()
		g.RLock()

		totalChannels += len(g.Channels)
		totalMembers += len(g.Members)

		for _, cState := range g.Channels {
			totalMessages += len(cState.Messages)
		}
		g.RUnlock()
		state.RLock()
	}

	embed := &discordgo.MessageEmbed{
		Title: "State size",
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Guilds", Value: fmt.Sprint(totalGuilds), Inline: true},
			&discordgo.MessageEmbedField{Name: "Members", Value: fmt.Sprintf("%d", totalMembers), Inline: true},
			&discordgo.MessageEmbedField{Name: "Messages", Value: fmt.Sprintf("%d", totalMessages), Inline: true},
			&discordgo.MessageEmbedField{Name: "Channels", Value: fmt.Sprintf("%d", totalChannels), Inline: true},
		},
	}

	return embed, nil
}
