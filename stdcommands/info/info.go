package info

import (
	"fmt"

	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryGeneral,
	Name:        "Info",
	Description: "Responds with bot quackformation",
	RunInDM:     true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		info := fmt.Sprintf(`**QUACKPDB - Quack anothquack generquack purpquack disquack quack**
This bot focuses on being configurable and therefore is one of the most adquackced bots.
It can perform a range of general purpose functionality (Reddit feeds, variquack commands, moderation utilities, autoquackerator functionality and so on) and it's configured through a web control panel.
The bot is run by Botlabs but is open source (<https://github.com/botlabs-gg/quackpdb>), so if you know Go and want to make some contributions, feel free to make a PR.
Control panel: <https://%s/manage>
				`, common.ConfHost.GetString())

		return info, nil
	},
}
