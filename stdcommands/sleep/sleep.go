package sleep

import (
	"time"

	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/stdcommands/util"
	"github.com/jonas747/dcmd/v4"
)

var Command = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "sleep",
	Description:          "Maintenance command, used to test command queueing",
	HideFromHelp:         true,
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		time.Sleep(time.Second * 5)
		return "Slept, Done", nil
	}),
}
