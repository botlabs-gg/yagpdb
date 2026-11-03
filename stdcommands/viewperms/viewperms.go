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
		{Name: "channel", Type: &dcmd.ChannelArg{AllowUnknown: true}},
	},
	RunFunc: func(data *dcmd.Data) (any, error) {
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

		if data.GuildData.GS.GetChannelOrThread(cid) == nil {
			return hiddenChannelPerms(data, target, cid), nil
		}

		perms, err := data.GuildData.GS.GetMemberPermissions(cid, target.User.ID, target.Member.Roles)
		if err != nil {
			return "Unable to calculate perms", err
		}

		return formatPerms(target, cid, perms), nil
	},
}

// the bot cannot see the overwrites of channels it cannot view, but slash commands resolve the permissions of the invoking member and the bot
func hiddenChannelPerms(data *dcmd.Data, target *dstate.MemberState, channelID int64) string {
	if data.TriggerType != dcmd.TriggerTypeSlashCommands {
		return "I can't see that channel, use the /viewperms slash command to check permissions in it"
	}

	resolved := data.SlashCommandTriggerData.Interaction.DataCommand.Resolved.Channels[channelID]
	if resolved == nil {
		return "I can't see that channel"
	}

	switch target.User.ID {
	case data.GuildData.MS.User.ID:
		return formatPerms(target, channelID, resolved.Permissions)
	case common.BotUser.ID:
		return formatPerms(target, channelID, resolved.AppPermissions)
	default:
		return "I can't see that channel, so I can only show your permissions or mine in it"
	}
}

func formatPerms(target *dstate.MemberState, channelID int64, perms int64) string {
	humanized := common.HumanizePermissions(perms)
	return fmt.Sprintf("Perms of %s in <#%d>\n`%d`\n%s", target.User.Username, channelID, perms, strings.Join(humanized, ", "))
}
