package customembed

import (
	"encoding/json"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var yagCommand = commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "CustomEmbed",
	Aliases:      []string{"ce"},
	Description:  "Creates an embed from what you give it in json form: https://discordapp.com/developers/docs/resources/channel#embed-object",
	RequiredArgs: 1,
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

func Cmd() util.Command {
	return &cmd{}
}

type cmd struct {
	util.BaseCmd
}

func (c cmd) YAGCommand() *commands.YAGCommand {
	return &yagCommand
}
