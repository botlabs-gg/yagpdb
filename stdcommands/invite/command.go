package invite

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var yagCommand = commands.YAGCommand{
	CmdCategory: commands.CategoryGeneral,
	Name:        "Invite",
	Aliases:     []string{"inv", "i"},
	Description: "Responds with bot invite link",
	RunInDM:     true,

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		return "Please add the bot through the websie\nhttps://" + common.Conf.Host, nil
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
