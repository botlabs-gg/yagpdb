package listflags

import (
	"strings"

	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/common/featureflags"
	"github.com/botlabs-gg/yagpdb/stdcommands/util"
	"github.com/jonas747/dcmd/v4"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "listflags",
	Description:          ";))",
	HideFromHelp:         true,
	RequiredArgs:         0,
	Arguments: []*dcmd.ArgDef{
		{Name: "server", Type: dcmd.BigInt},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		target := data.GuildData.GS.ID
		if data.Args[0].Int64() != 0 {
			target = data.Args[0].Int64()
		}

		flags, err := featureflags.GetGuildFlags(target)
		if err != nil {
			return nil, err
		}

		return "Feature flags: ```\n" + strings.Join(flags, "\n") + "```", nil
	}),
}
