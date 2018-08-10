package info

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryGeneral,
	Name:        "Info",
	Description: "Responds with bot information",
	RunInDM:     true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		info := fmt.Sprintf(`**YAGPDB - Yet Another General Purpose Discord Bot**
This bot focuses on being configurable and therefore is one of the more advanced bots.
It can perform a range of general purpose functionality (reddit feeds, various commands, moderation utilities, automoderator functionality and so on) and it's configured through a web control panel.
I'm currently being run and developed by jonas747#0001 (105487308693757952) but the bot is open source (<https://github.com/jonas747/yagpdb>), so if you know go and want to make some contributions, DM me.
Control panel: <https://%s/manage>
				`, common.Conf.Host)

		return info, nil
	},
}
