package mentionrole

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"strings"
	"time"
)

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryTool,
	Name:            "MentionRole",
	Aliases:         []string{"mrole"},
	Description:     "Sets a role to mentionable, mentions the role, and then sets it back",
	LongDescription: "Requires the manage roles permission and the bot being above the mentioned role",
	RequiredArgs:    1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Role", Type: dcmd.String},
	},
	RunFunc: cmdFuncMentionRole,
}

func cmdFuncMentionRole(data *dcmd.Data) (interface{}, error) {
	if ok, err := bot.AdminOrPerm(discordgo.PermissionManageRoles, data.Msg.Author.ID, data.CS.ID); err != nil {
		return "Failed checking perms", err
	} else if !ok {
		return "You need manage server perms to use this commmand", nil
	}

	var role *discordgo.Role
	data.GS.RLock()
	defer data.GS.RUnlock()
	for _, r := range data.GS.Guild.Roles {
		if strings.EqualFold(r.Name, data.Args[0].Str()) {
			role = r
			break
		}
	}

	if role == nil {
		return "No role with the name `" + data.Args[0].Str() + "` found", nil
	}

	_, err := common.BotSession.GuildRoleEdit(data.GS.ID, role.ID, role.Name, role.Color, role.Hoist, role.Permissions, true)
	if err != nil {
		if _, dErr := common.DiscordError(err); dErr != "" {
			return "Failed updating role, discord responded with: " + dErr, err
		} else {
			return "An unknown error occured updating the role", err
		}
	}

	_, err = common.BotSession.ChannelMessageSend(data.CS.ID, "<@&"+discordgo.StrID(role.ID)+">")

	time.Sleep(time.Second * 2)

	common.BotSession.GuildRoleEdit(data.GS.ID, role.ID, role.Name, role.Color, role.Hoist, role.Permissions, false)
	return "", err
}
