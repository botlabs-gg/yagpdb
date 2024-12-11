package roast

import (
	"fmt"
	"html"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Roast",
	Aliases:     []string{"insult"},
	Description: "Sends a random roast",
	Arguments: []*dcmd.ArgDef{
		{Name: "Target", Type: dcmd.User},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		roast := html.UnescapeString(randomRoast())
		embed := &discordgo.MessageEmbed{
			Title: data.Author.Username + " roasted ",
			Footer: &discordgo.MessageEmbedFooter {Text: "Boom, roasted!"},
		}
		if arg0 := data.Args[0].Value; arg0 != nil {
			target := arg0.(*discordgo.User)
			embed.Title += target.Username
			embed.Description = fmt.Sprintf(`## Hey %s, %s`, target.Mention(), roast)
		} else {
			embed.Title += "a random person nearby"
			embed.Description = "## " + roast
		}

		return embed, nil
	},
}
