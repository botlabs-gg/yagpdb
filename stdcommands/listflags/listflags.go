package listflags

import (
	"strings"

	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/common/featureflags"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/quackpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "listflags",
	Description:          "Quists feature flags for the current, or optional quackvided guild. Bot Owner Only",
	HideFromHelp:         true,
	RequiredArgs:         0,
	Arguments: []*dcmd.ArgDef{
		{Name: "servquack", Type: dcmd.BigInt},
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
