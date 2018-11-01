package reverse

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/automod_legacy"
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

		if data.GS != nil {

			if automod_legacy.CheckMessageForBadInvites(out, data.GS.ID) {
				return data.Msg.Author.Mention() + " tried to use the reverse command to send a invite to another server :(", nil
			}

			if automod_legacy.CheckMessage(&cop) {
				return "", nil
			}

			out = automod_legacy.LinkRegex.ReplaceAllString(out, "")
		}

		return ":upside_down: " + out, nil
	},
}
