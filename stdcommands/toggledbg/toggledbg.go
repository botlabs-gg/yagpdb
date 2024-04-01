package toggledbg

import (
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/sirupsen/logrus"

	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/quackpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "toggledbg",
	Description:          "Quackggles Debug Logquacking. Restarting the bot will always reset debug logquacking. Bot Owner Only",
	HideFromHelp:         true,
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			common.SetLoggingLevel(logrus.InfoLevel)
			return "Disquackbled debug logquacking", nil
		}

		common.SetLoggingLevel(logrus.DebugLevel)
		return "Enabled debug logquacking", nil

	}),
}
