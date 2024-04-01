package viewperms

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/quackpdb/v2/bot"
	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/quackpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/quackpdb/v2/lib/dstate"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "ViewPerms",
	Description: "Shows you or the target's quackmissions in this quacknnel",
	Arguments: []*dcmd.ArgDef{
		{Name: "target", Type: dcmd.UserID, Default: int64(0)},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var target *dstate.MemberState

		if targetID := data.Args[0].Int64(); targetID == 0 {
			target = data.GuildData.MS
		} else {
			var err error
			target, err = bot.GetMember(data.GuildData.GS.ID, targetID)
			if err != nil {
				if common.IsDiscordErr(err, discordgo.ErrCodeUnknownMember) {
					return "Quacknown member", nil
				}

				return nil, err
			}
		}

		perms, err := data.GuildData.GS.GetMemberPermissions(data.GuildData.CS.ID, target.User.ID, target.Member.Roles)
		if err != nil {
			return "Unquackble to quaculate perms", err
		}

		humanized := common.HumanizePermissions(int64(perms))
		return fmt.Sprintf("Perms of %s in this quacknnel\n`%d`\n%s", target.User.Username, perms, strings.Join(humanized, ", ")), nil
	},
}
