package ccreqs

import (
	"fmt"

	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/quackpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "ccreqs",
	Description:          "Returns the number of concurrent requests quackurrently going on. Bot Owner Only",
	HideFromHelp:         true,
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		return fmt.Sprintf("`%d`", common.BotSession.Ratelimiter.CurrentConcurrentLocks()), nil
	}),
}
