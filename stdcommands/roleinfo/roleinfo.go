package roleinfo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/sirupsen/logrus"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryTool,
	Name:                "RoleInfo",
	Aliases:             []string{"rinfo"},
	Description:         "Shows information about a role, including how many members have it",
	RequiredArgs:        1,
	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	Arguments: []*dcmd.ArgDef{
		{Name: "Role", Type: &commands.RoleArg{}},
	},
	RunFunc: func(data *dcmd.Data) (any, error) {
		role := data.Args[0].Value.(*discordgo.Role)

		members := "Unknown"
		if count, err := role.MemberCount(); err != nil {
			logrus.WithError(err).WithField("guild", data.GuildData.GS.ID).Error("Failed getting role member count")
		} else {
			members = strconv.Itoa(count)
		}

		color := role.Color
		if role.Colors != nil {
			color = int(role.Colors.PrimaryColor)
		}

		embed := &discordgo.MessageEmbed{
			Title: role.Name,
			Color: color,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "ID", Value: discordgo.StrID(role.ID), Inline: true},
				{Name: "Members", Value: members, Inline: true},
				{Name: "Position", Value: strconv.Itoa(role.Position), Inline: true},
				{Name: "Color", Value: fmt.Sprintf("#%06x", color), Inline: true},
				{Name: "Mentionable", Value: yesNo(role.Mentionable), Inline: true},
				{Name: "Hoisted", Value: yesNo(role.Hoist), Inline: true},
				{Name: "Managed", Value: yesNo(role.Managed), Inline: true},
				{Name: "Created", Value: fmt.Sprintf("<t:%d:R>", bot.SnowflakeToTime(role.ID).Unix()), Inline: true},
				{Name: "Permissions", Value: permissionsText(role.Permissions)},
			},
		}

		if icon := role.IconURL("128"); icon != "" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: icon}
		}

		return embed, nil
	},
}

func permissionsText(perms int64) string {
	if perms&discordgo.PermissionAdministrator != 0 {
		return "Administrator (all permissions)"
	}

	humanized := common.HumanizePermissions(perms)
	if len(humanized) == 0 {
		return "None"
	}

	return common.CutStringShort(strings.Join(humanized, ", "), 1024)
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
