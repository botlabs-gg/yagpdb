package viewperms

import (
	"fmt"
	"strings"

	"github.com/jonas747/dcmd/v3"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v3"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "ViewPerms",
	Description: "Shows you or the target's permissions in this channel",
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
					return "Unknown member", nil
				}

				return nil, err
			}
		}

		perms, err := data.GuildData.GS.GetMemberPermissions(data.GuildData.CS.ID, target.User.ID, target.Member.Roles)
		if err != nil {
			return "Unable to calculate perms", err
		}

		humanized := common.HumanizePermissions(int64(perms))
		return fmt.Sprintf("Perms of %s in this channel\n`%d`\n%s", target.User.Username, perms, strings.Join(humanized, ", ")), nil
	},
}
