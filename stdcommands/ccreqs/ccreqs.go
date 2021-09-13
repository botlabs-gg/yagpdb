package ccreqs

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/stdcommands/util"
	"github.com/jonas747/dcmd/v4"
)

var Command = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "ccreqs",
	Description:          "Returns the number of concurrent requests currently going on",
	HideFromHelp:         true,
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		return fmt.Sprintf("`%d`", common.BotSession.Ratelimiter.CurrentConcurrentLocks()), nil
	}),
}
