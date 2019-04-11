package setstatus

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "setstatus",
	Description:          "Sets the bot's status and streaming url",
	HideFromHelp:         true,
	Arguments: []*dcmd.ArgDef{
		{Name: "status", Type: dcmd.String, Default: ""},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Switch: "url", Name: "streaming url", Type: dcmd.String, Default: ""},
	},
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		streamingURL := data.Switch("url").Str()
		bot.SetStatus(streamingURL, data.Args[0].Str())
		return "Doneso", nil
	}),
}
