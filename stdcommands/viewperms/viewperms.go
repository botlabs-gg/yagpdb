package viewperms

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryDebug,
	Name:                "ViewPerms",
	Description:         "Shows you or the target's permissions in a given channel (default current channel)",
	SlashCommandEnabled: true,
	Arguments: []*dcmd.ArgDef{
		{Name: "target", Type: dcmd.UserID, Default: int64(0)},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "channel", Type: dcmd.Channel},
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

		cid := data.ChannelID
		if c := data.Switch("channel"); c.Value != nil {
			cid = c.Value.(*dstate.ChannelState).ID
		}

		perms, err := data.GuildData.GS.GetMemberPermissions(cid, target.User.ID, target.Member.Roles)
		if err != nil {
			return "Unable to calculate perms", err
		}

		humanized := common.HumanizePermissions(int64(perms))
		return fmt.Sprintf("Perms of %s in <#%d>\n`%d`\n%s", target.User.Username, cid, perms, strings.Join(humanized, ", ")), nil
	},
}
