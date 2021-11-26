package throw

import (
	"fmt"
	"math/rand"

	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/jonas747/dcmd/v4"
	"github.com/jonas747/discordgo/v2"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Throw",
	Description: "Throwing things is cool.",
	Arguments: []*dcmd.ArgDef{
		{Name: "Target", Type: dcmd.User},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		target := "a random person nearby"
		if data.Args[0].Value != nil {
			target = data.Args[0].Value.(*discordgo.User).Username
		}

		resp := ""

		rng := rand.Intn(100)
		if rng < 5 {
			resp = fmt.Sprintf("TRIPLE THROW! Threw **%s**, **%s** and **%s** at **%s**", randomThing(), randomThing(), randomThing(), target)
		} else if rng < 15 {
			resp = fmt.Sprintf("DOUBLE THROW! Threw **%s** and **%s** at **%s**", randomThing(), randomThing(), target)
		} else {
			resp = fmt.Sprintf("Threw **%s** at **%s**", randomThing(), target)
		}

		return resp, nil
	},
}
