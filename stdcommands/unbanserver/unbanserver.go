package unbanserver

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "unbanserver",
	Description:          ";))",
	HideFromHelp:         true,
	RequiredArgs:         1,
	Arguments: []*dcmd.ArgDef{
		{Name: "server", Type: dcmd.String},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {

		var unbanned bool
		err := common.RedisPool.Do(retryableredis.Cmd(&unbanned, "SREM", "banned_servers", data.Args[0].Str()))
		if err != nil {
			return nil, err
		}

		if !unbanned {
			return "Server wasnt banned", nil
		}

		return "Unbanned server", nil
	}),
}
