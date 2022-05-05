package rolecommands

import (
	"context"
	"database/sql"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	schEvtsModels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands/models"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func (p *Plugin) AddCommands() {
	const msgIDDocs = "To get the id of a message you have to turn on developer mode in Discord's appearances settings then right click the message and copy id."

	categoryRoleMenu := &dcmd.Category{
		Name:        "Rolemenu",
		Description: "Rolemenu commands",
		HelpEmoji:   "🔘",
		EmbedColor:  0x42b9f4,
	}

	commands.AddRootCommands(p,
		&commands.YAGCommand{
			CmdCategory: commands.CategoryTool,
			Name:        "Role",
			Description: "Toggle a role on yourself or list all available roles, they have to be set up in the control panel first, under 'rolecommands' ",
			Arguments: []*dcmd.ArgDef{
				{Name: "Role", Type: dcmd.String},
			},
			SlashCommandEnabled: true,
			DefaultEnabled:      true,
			RunFunc:             CmdFuncRole,
		})

	cmdCreate := &commands.YAGCommand{
		Name:                "Create",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"c"},
		Description:         "Set up a role menu.",
		LongDescription:     "Specify a message with -m to use an existing message instead of having the bot make one\n\n" + msgIDDocs,
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
		RequiredArgs:        1,
		Arguments: []*dcmd.ArgDef{
			{Name: "Group", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "m", Help: "Message ID", Type: dcmd.BigInt},
			{Name: "nodm", Help: "Disable DM"},
			{Name: "rr", Help: "Remove role on reaction removed"},
			{Name: "skip", Help: "Number of roles to skip", Default: 0, Type: dcmd.Int},
		},
		RunFunc: cmdFuncRoleMenuCreate,
	}

	cmdRemoveRoleMenu := &commands.YAGCommand{
		Name:                "Remove",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"rm"},
		Description:         "Removes a rolemenu from a message.",
		LongDescription:     "The message won't be deleted and the bot will not do anything with reactions on that message\n\n" + msgIDDocs,
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
		RequiredArgs:        1,
		Arguments: []*dcmd.ArgDef{
			{Name: "Message-ID", Type: dcmd.BigInt},
		},
		RunFunc: cmdFuncRoleMenuRemove,
	}

	cmdUpdate := &commands.YAGCommand{
		Name:                "Update",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"u"},
		Description:         "Updates a rolemenu, toggling the provided flags and adding missing options, aswell as updating the order.",
		LongDescription:     "\n\n" + msgIDDocs,
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
		RequiredArgs:        1,
		Arguments: []*dcmd.ArgDef{
			{Name: "Message-ID", Type: dcmd.BigInt},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "nodm", Help: "Disable DM"},
			{Name: "rr", Help: "Remove role on reaction removed"},
		},
		RunFunc: cmdFuncRoleMenuUpdate,
	}

	cmdResetReactions := &commands.YAGCommand{
		Name:                "ResetReactions",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"reset"},
		Description:         "Removes all reactions on the specified menu message and re-adds them.",
		LongDescription:     "Can be used to fix the order after updating it.\n\n" + msgIDDocs,
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
		RequiredArgs:        1,
		Arguments: []*dcmd.ArgDef{
			{Name: "Message-ID", Type: dcmd.BigInt},
		},
		RunFunc: cmdFuncRoleMenuResetReactions,
	}

	cmdEditOption := &commands.YAGCommand{
		Name:                "EditOption",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"edit"},
		Description:         "Allows you to reassign the emoji of an option, tip: use ResetReactions afterwards.",
		LongDescription:     "\n\n" + msgIDDocs,
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
		RequiredArgs:        1,
		Arguments: []*dcmd.ArgDef{
			{Name: "Message-ID", Type: dcmd.BigInt},
		},
		RunFunc: cmdFuncRoleMenuEditOption,
	}

	cmdFinishSetup := &commands.YAGCommand{
		Name:                "Complete",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"finish"},
		Description:         "Marks the menu as done.",
		LongDescription:     "\n\n" + msgIDDocs,
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
		RequiredArgs:        1,
		Arguments: []*dcmd.ArgDef{
			{Name: "Message-ID", Type: dcmd.BigInt},
		},
		RunFunc: cmdFuncRoleMenuComplete,
	}

	cmdListGroups := &commands.YAGCommand{
		Name:                "Listgroups",
		CmdCategory:         categoryRoleMenu,
		Aliases:             []string{"list", "groups"},
		Description:         "Lists all role groups",
		RequireDiscordPerms: []int64{discordgo.PermissionManageGuild},
		RunFunc:             cmdFuncRoleMenuListGroups,
	}

	menuContainer, t := commands.CommandSystem.Root.Sub("RoleMenu", "rmenu")
	t.SetEnabledInThreads(false)
	menuContainer.Description = "Command for managing role menus"

	const notFoundMessage = "Unknown rolemenu command, if you've used this before it was recently revamped.\nTry almost the same command but `rolemenu create ...` and `rolemenu update ...` instead (replace '...' with the rest of the command).\nSee `help rolemenu` for all rolemenu commands."
	menuContainer.NotFound = commands.CommonContainerNotFoundHandler(menuContainer, notFoundMessage)

	menuContainer.AddCommand(cmdCreate, cmdCreate.GetTrigger())
	menuContainer.AddCommand(cmdRemoveRoleMenu, cmdRemoveRoleMenu.GetTrigger())
	menuContainer.AddCommand(cmdUpdate, cmdUpdate.GetTrigger())
	menuContainer.AddCommand(cmdResetReactions, cmdResetReactions.GetTrigger())
	menuContainer.AddCommand(cmdEditOption, cmdEditOption.GetTrigger())
	menuContainer.AddCommand(cmdFinishSetup, cmdFinishSetup.GetTrigger())
	menuContainer.AddCommand(cmdListGroups, cmdListGroups.GetTrigger())
	commands.RegisterSlashCommandsContainer(menuContainer, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}

type ScheduledMemberRoleRemoveData struct {
	GuildID int64 `json:"guild_id"`
	GroupID int64 `json:"group_id"`
	UserID  int64 `json:"user_id"`
	RoleID  int64 `json:"role_id"`
}

type ScheduledEventUpdateMenuMessageData struct {
	GuildID   int64 `json:"guild_id"`
	MessageID int64 `json:"message_id"`
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, handleReactionAddRemove, eventsystem.EventMessageReactionAdd, eventsystem.EventMessageReactionRemove)
	eventsystem.AddHandlerAsyncLastLegacy(p, handleMessageRemove, eventsystem.EventMessageDelete, eventsystem.EventMessageDeleteBulk)

	scheduledevents2.RegisterHandler("remove_member_role", ScheduledMemberRoleRemoveData{}, handleRemoveMemberRole)
	scheduledevents2.RegisterHandler("rolemenu_update_message", ScheduledEventUpdateMenuMessageData{}, handleUpdateRolemenuMessage)

	pubsub.AddHandler("role_commands_evict_menus", func(evt *pubsub.Event) {
		ClearRolemenuCache(evt.TargetGuildInt)
		recentMenusTracker.GuildReset(evt.TargetGuildInt)
	}, nil)
}

func CmdFuncRole(parsed *dcmd.Data) (interface{}, error) {
	if parsed.Args[0].Value == nil {
		return CmdFuncListCommands(parsed)
	}

	given, err := FindToggleRole(parsed.Context(), parsed.GuildData.MS, parsed.Args[0].Str())
	if err != nil {
		if err == sql.ErrNoRows {
			resp, err := CmdFuncListCommands(parsed)
			if v, ok := resp.(string); ok {
				return "Role not found, " + v, err
			}

			return resp, err
		}

		return HumanizeAssignError(parsed.GuildData.GS, err)
	}

	go analytics.RecordActiveUnit(parsed.GuildData.GS.ID, &Plugin{}, "cmd_used")

	if given {
		return "Gave you the role!", nil
	}

	return "Took away your role!", nil
}

func HumanizeAssignError(guild *dstate.GuildSet, err error) (string, error) {
	if IsRoleCommandError(err) {
		if roleError, ok := err.(*RoleError); ok {
			return roleError.PrettyError(guild.Roles), nil
		}
		return err.Error(), nil
	}

	if code, msg := common.DiscordError(err); code != 0 {
		if code == discordgo.ErrCodeMissingPermissions {
			return "The bot is below the role, contact the server admin", err
		} else if code == discordgo.ErrCodeMissingAccess {
			return "Bot does not have enough permissions to assign you this role, contact the server admin", err
		}

		return "An error occurred while assigning the role: " + msg, err
	}

	return "An error occurred while assigning the role", err

}

func CmdFuncListCommands(parsed *dcmd.Data) (interface{}, error) {
	_, grouped, ungrouped, err := GetAllRoleCommandsSorted(parsed.Context(), parsed.GuildData.GS.ID)
	if err != nil {
		return "Failed retrieving role commands", err
	}

	output := "Here is a list of available roles:\n"

	didListCommands := false
	for group, cmds := range grouped {
		if len(cmds) < 1 {
			continue
		}
		didListCommands = true

		output += "**" + group.Name + "**\n"
		output += StringCommands(cmds)
		output += "\n"
	}

	if len(ungrouped) > 0 {
		didListCommands = true

		output += "**Ungrouped roles**\n"
		output += StringCommands(ungrouped)
	}

	if !didListCommands {
		output += "No role commands (self assignable roles) set up. You can set them up in the control panel."
	}

	return output, nil
}

// StringCommands pretty formats a bunch of commands into  a string
func StringCommands(cmds []*models.RoleCommand) string {
	stringedCommands := make([]int64, 0, len(cmds))

	output := "```\n"

	for _, cmd := range cmds {
		if common.ContainsInt64Slice(stringedCommands, cmd.Role) {
			continue
		}

		output += cmd.Name
		// Check for duplicate roles
		for _, cmd2 := range cmds {
			if cmd.Role == cmd2.Role && cmd.Name != cmd2.Name {
				output += "/ " + cmd2.Name
			}
		}
		output += "\n"

		stringedCommands = append(stringedCommands, cmd.Role)
	}

	return output + "```\n"
}

func handleUpdateRolemenuMessage(evt *schEvtsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*ScheduledEventUpdateMenuMessageData)

	fullMenu, err := FindRolemenuFull(context.Background(), dataCast.MessageID, dataCast.GuildID)
	if err != nil {
		return false, err
	}

	err = UpdateRoleMenuMessage(context.Background(), fullMenu)
	if err != nil {
		return false, err
	}

	return false, nil
}

func handleRemoveMemberRole(evt *schEvtsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*ScheduledMemberRoleRemoveData)
	err = common.BotSession.GuildMemberRoleRemove(dataCast.GuildID, dataCast.UserID, dataCast.RoleID)
	if err != nil {
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	// remove the reaction
	menus, err := models.RoleMenus(
		qm.Where("role_group_id = ? AND guild_id =?", dataCast.GroupID, dataCast.GuildID),
		qm.OrderBy("message_id desc"),
		qm.Limit(10),
		qm.Load("RoleMenuOptions.RoleCommand")).AllG(context.Background())
	if err != nil {
		return false, err
	}

OUTER:
	for _, v := range menus {
		for _, opt := range v.R.RoleMenuOptions {
			if opt.R.RoleCommand.Role == dataCast.RoleID {
				// remove it
				emoji := opt.UnicodeEmoji
				if opt.EmojiID != 0 {
					emoji = "aaa:" + discordgo.StrID(opt.EmojiID)
				}

				err := common.BotSession.MessageReactionRemove(v.ChannelID, v.MessageID, emoji, dataCast.UserID)
				common.LogIgnoreError(err, "rolecommands: failed removing reaction", logrus.Fields{"guild": dataCast.GuildID, "user": dataCast.UserID, "emoji": emoji})
				continue OUTER
			}
		}
	}

	return scheduledevents2.CheckDiscordErrRetry(err), err
}

type CacheKey struct {
	GuildID   int64
	MessageID int64
}

var menuCache = common.CacheSet.RegisterSlot("rolecommands_menus", nil, int64(0))

func GetRolemenuCached(ctx context.Context, gs *dstate.GuildSet, messageID int64) (*models.RoleMenu, error) {
	result, err := menuCache.GetCustomFetch(CacheKey{
		GuildID:   gs.ID,
		MessageID: messageID,
	}, func(key interface{}) (interface{}, error) {
		menu, err := FindRolemenuFull(ctx, messageID, gs.ID)
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
			return nil, nil
		}

		return menu, nil
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.(*models.RoleMenu), nil
}

func ClearRolemenuCache(gID int64) {
	menuCache.DeleteFunc(func(key interface{}, value interface{}) bool {
		keyCast := key.(CacheKey)
		return keyCast.GuildID == gID
	})
}
