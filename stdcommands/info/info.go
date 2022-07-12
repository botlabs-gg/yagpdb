package info

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryGeneral,
	Name:        "Info",
	Description: "Responds with bot information",
	RunInDM:     true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		info := fmt.Sprintf(`**YAGPDB - Yet Another General Purpose Discord Bot**
This bot focuses on being configurable and therefore is one of the most advanced bots.
It can perform a range of general purpose functionality (Reddit feeds, various commands, moderation utilities, automoderator functionality and so on) and it's configured through a web control panel.
The bot is run by Botlabs but is open source (<https://github.com/botlabs-gg/yagpdb>), so if you know Go and want to make some contributions, feel free to make a PR.
Control panel: <https://%s/manage>
				`, common.ConfHost.GetString())

		return info, nil
	},
}
