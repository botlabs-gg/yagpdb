package motivation

import (
	"html"
	"math/rand"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "Motivation",
	Aliases:             []string{"motivate"},
	Description:         "Sends a random motivation",
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		embed := &discordgo.MessageEmbed{}
		embed.Title = "Here is your motivational quote"
		embed.Description = html.UnescapeString(randommotivation())
		embed.Color = int(rand.Int63n(16777215))
		return embed, nil
	},
}
