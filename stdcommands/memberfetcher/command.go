package memberfetcher

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var yagCommand = commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "MemberFetcher",
	Aliases:              []string{"memfetch"},
	Description:          "Shows the current status of the member fetcher",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		fetching, notFetching := bot.MemberFetcher.Status()
		return fmt.Sprintf("Fetching: `%d`, Not fetching: `%d`", fetching, notFetching), nil
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
