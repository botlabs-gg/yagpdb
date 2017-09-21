package stdcommands

import (
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var cmdInvite = &commands.CustomCommand{
	Category: commands.CategoryGeneral,
	Command: &commandsystem.Command{
		Name:        "Invite",
		Aliases:     []string{"inv", "i"},
		Description: "Responds with bot invite link",
		RunInDm:     true,

		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "Please add the bot through the websie\nhttps://" + common.Conf.Host, nil
		},
	},
}

var cmdInfo = &commands.CustomCommand{
	Category: commands.CategoryGeneral,
	Command: &commandsystem.Command{
		Name:        "Info",
		Description: "Responds with bot information",
		RunInDm:     true,
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			const info = `**YAGPDB - Yet Another General Purpose Discord Bot**
This bot focuses on being configurable and therefore is one of the more advanced bots.
It can perform a range of general purpose functionality (reddit feeds, various commands, moderation utilities, automoderator functionality and so on) and it's configured through a web control panel.
I'm currently being ran and developed by jonas747#3124 (105487308693757952) but the bot is open source (<https://github.com/jonas747/yagpdb>), so if you know go and want to make some contributions, DM me.
Control panel: <https://yagpdb.xyz/manage>
				`
			return info, nil
		},
	},
}
