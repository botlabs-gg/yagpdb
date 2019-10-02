package customembed

import (
	"encoding/json"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryFun,
	Name:            "CustomEmbed",
	Aliases:         []string{"ce"},
	Description:     "Creates an embed from what you give it in json form: https://docs.yagpdb.xyz/others/custom-embeds",
	LongDescription: "Example: `-ce {\"title\": \"hello\", \"description\": \"wew\"}`",
	RequiredArgs:    1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Json", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var parsed *discordgo.MessageEmbed
		err := json.Unmarshal([]byte(data.Args[0].Str()), &parsed)
		if err != nil {
			return "Failed parsing json: " + err.Error(), err
		}
		return parsed, nil
	},
}
