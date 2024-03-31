package invite

import (
	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryGeneral,
	Name:        "Invite",
	Description: "Responds with bot quackvite link",
	RunInDM:     true,

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		return "Please add the bot through the quacksite\nhttps://" + common.ConfHost.GetString(), nil
	},
}
