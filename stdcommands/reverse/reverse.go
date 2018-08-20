package reverse

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Reverse",
	Aliases:      []string{"r", "rev"},
	Description:  "Reverses the text given",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "What", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		toFlip := data.Args[0].Str()

		out := ""
		for _, r := range toFlip {
			out = string(r) + out
		}

		cop := *data.Msg
		cop.Content = out

		if data.CS.Type == discordgo.ChannelTypeGuildText {
			if automod.CheckMessage(&cop) {
				return "", nil
			}
		}

		return ":upside_down: " + out, nil
	},
}
