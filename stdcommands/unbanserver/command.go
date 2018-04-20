package unbanserver

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"github.com/mediocregopher/radix.v2/redis"
)

var yagCommand = commands.YAGCommand{
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
		client := data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
		unbanned, err := client.Cmd("SREM", "banned_servers", data.Args[0].Str()).Int()
		if err != nil {
			return err, err
		}

		if unbanned < 1 {
			return "Server wasnt banned", nil
		}

		return "Unbanned server", nil
	}),
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
