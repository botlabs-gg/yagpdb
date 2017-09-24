package rolecommands

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"gopkg.in/src-d/go-kallax.v1"
	"sort"
	"strconv"
)

func CmdFuncRoleMenu(parsed *commandsystem.ExecData) (interface{}, error) {
	member, err := bot.GetMember(parsed.Guild.ID(), parsed.Message.Author.ID)
	if err != nil {
		return "Failed retrieving member", err
	}

	ok, err := bot.AdminOrPerm(discordgo.PermissionManageServer, member.User.ID, parsed.Channel.ID())
	if err != nil {
		return "Failed checkign your perms", err
	}

	if !ok {
		return "You do not have the proper permissions (Manage Server) to create a role menu", nil
	}

	group, err := groupStore.FindOne(NewRoleGroupQuery().FindByGuildID(kallax.Eq, common.MustParseInt(parsed.Guild.ID())).Where(kallax.Ilike(Schema.RoleGroup.Name, parsed.Args[0].Str())))
	if err != nil {
		if err == kallax.ErrNotFound {
			return "Did not find the role command group specified, make sure you types it right", nil
		}

		return "Failed retrieving the group", err
	}

	// set up the message if not provided
	msg, err := common.BotSession.ChannelMessageSend(parsed.Channel.ID(), "Role menu\nSetting up...")
	if err != nil {
		_, dErr := common.DiscordError(err)
		errStr := "Failed creating the menu message, check the permissions on the channel"
		if dErr != "" {
			errStr += ", discord respondedn with: " + errStr
		}
		return errStr, err
	}

	model := &RoleMenu{
		MessageID: common.MustParseInt(msg.ID),
		GuildID:   common.MustParseInt(parsed.Guild.ID()),
		OwnerID:   common.MustParseInt(parsed.Message.Author.ID),
		ChannelID: common.MustParseInt(parsed.Message.ChannelID),

		OwnMessage: true,
		Group:      group,
	}

	err = roleMenuStore.Insert(model)
	if err != nil {
		return "Failed setting up menu", err
	}

	resp, err := model.NextSetupStep(true)

	content := msg.Content + "\n" + resp
	_, err = common.BotSession.ChannelMessageEdit(parsed.Channel.ID(), msg.ID, content)
	return "", err
}

func (rm *RoleMenu) NextSetupStep(first bool) (resp string, err error) {
	commands, err := cmdStore.FindAll(NewRoleCommandQuery().FindByGroup(rm.Group.ID).Order(kallax.Desc(Schema.RoleCommand.ID)))
	if err != nil {
		return "Failed fetching commands for role group", err
	}

	var nextCmd *RoleCommand

	sort.Slice(commands, RoleCommandsLessFunc(commands))

OUTER:
	for _, cmd := range commands {
		for _, option := range rm.Options {
			if cmd.ID == option.RoleCmd.ID {
				continue OUTER
			}
		}

		// New command is cmd
		nextCmd = cmd
		break
	}

	rm.Options = nil

	if nextCmd == nil {
		rm.State = RoleMenuStateDone
		roleMenuStore.Update(rm, Schema.RoleMenu.State)
		return "Done setting up!", nil
	}

	rm.NextRoleCommand = nextCmd
	roleMenuStore.Debug().Update(rm, Schema.RoleMenu.NextRoleCommandFK)
	if first {
		return "**Start setup**: React with the emoji for the role command: `" + nextCmd.Name + "` on the this message\nNote: the bot has to be on the server where the emoji is from, otherwise it wont be able to use it", nil
	}
	return "**Continue setup**: React with the emoji for the role command: `" + nextCmd.Name + "` on the original message\nNote: the bot has to be on the server where the emoji is from, otherwise it wont be able to use it", nil
}

func (rm *RoleMenu) UpdateMenuMessage() error {
	newMsg := "**Role Menu**: react to give yourself a role\n\n"

	for _, option := range rm.Options {
		// Fetch the roel command if missing
		if option.RoleCmd == nil {
			cmdId, err := option.Value("role_command_id")
			if err != nil {
				return err
			}
			// Not very pretty, but a limitation of kallax atm...
			cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByID(int64(*(cmdId.(*kallax.NumericID)))))
			if err != nil {
				return err
			}
			option.RoleCmd = cmd
		}
	}

	sort.Slice(rm.Options, RoleMenuOptionLessFunc(rm.Options))

	for _, option := range rm.Options {
		emoji := option.UnicodeEmoji
		if option.EmojiID != 0 {
			emoji = fmt.Sprintf("<:yagpdb:%d>", option.EmojiID)
		}
		newMsg += fmt.Sprintf("%s : `%s`\n\n", emoji, option.RoleCmd.Name)
	}

	_, err := common.BotSession.ChannelMessageEdit(strconv.FormatInt(rm.ChannelID, 10), strconv.FormatInt(rm.MessageID, 10), newMsg)
	return err
}

func (rm *RoleMenu) ContinueSetup(ra *discordgo.MessageReactionAdd) (resp string, err error) {
	if common.MustParseInt(ra.UserID) != rm.OwnerID {
		common.BotSession.MessageReactionRemove(ra.ChannelID, ra.MessageID, ra.Emoji.APIName(), ra.UserID)
		return "This menu is still being set up, wait until the owner of this menu is done.", nil
	}

	parsedID := int64(0)
	if ra.Emoji.ID != "" {
		parsedID = common.MustParseInt(ra.Emoji.ID)
	}

	// Make sure this emoji isnt used to another option
	for _, option := range rm.Options {
		if ra.Emoji.ID != "" {
			if parsedID == option.EmojiID && option.EmojiID != 0 {
				return "Emoji already used for another option", nil
			}
		} else {
			if ra.Emoji.Name == option.UnicodeEmoji && option.UnicodeEmoji != "" {
				return "Emoji already used for another option", nil
			}
		}
	}

	model := &RoleMenuOption{
		Menu:    rm,
		RoleCmd: rm.NextRoleCommand,
		EmojiID: parsedID,
	}

	if ra.Emoji.ID == "" {
		model.UnicodeEmoji = ra.Emoji.Name
	}

	err = roleMenuOptionStore.Insert(model)
	if err != nil {
		return "Failed inserting option", err
	}

	err = common.BotSession.MessageReactionAdd(ra.ChannelID, ra.MessageID, ra.Emoji.APIName())
	if err != nil {
		logrus.WithError(err).WithField("emoji", ra.Emoji.APIName()).Error("Failed reacting")
	}

	rm.Options = append(rm.Options, model)
	err = rm.UpdateMenuMessage()
	if err != nil {
		return "Failed updating", err
	}

	return rm.NextSetupStep(false)
}

func handleReactionAdd(evt *eventsystem.EventData) {
	ra := evt.MessageReactionAdd
	if ra.UserID == common.BotUser.ID {
		return
	}

	menu, err := roleMenuStore.FindOne(NewRoleMenuQuery().FindByMessageID(common.MustParseInt(ra.MessageID)).WithOptions(nil).WithNextRoleCommand().WithGroup())
	if err != nil {
		if err != kallax.ErrNotFound {
			logrus.WithError(err).Error("RoleCommnadsMenu: Failed finding menu")
		}
		return
	}

	if menu.State == RoleMenuStateSettingUp {
		resp, err := menu.ContinueSetup(ra)
		if err != nil {
			logrus.WithError(err).Error("RoleCommnadsMenu: Failed continuing role menu setup")
		}

		if resp != "" {
			_, err = common.BotSession.ChannelMessageSend(ra.ChannelID, "Role menu setup: "+resp)
			if err != nil {
				logrus.WithError(err).Error("RoleCommnadsMenu: Failed sending new response")
			}
		}

		return
	}

	var option *RoleMenuOption
	if ra.Emoji.ID != "" {
		// This is a custom emoji
		parsedID := common.MustParseInt(ra.Emoji.ID)
		for _, v := range menu.Options {
			if v.EmojiID != 0 && v.EmojiID == parsedID {
				option = v
			}
		}
	} else {
		// Unicode emoji
		for _, v := range menu.Options {
			if v.UnicodeEmoji == ra.Emoji.Name && v.EmojiID == 0 {
				option = v
			}
		}
	}

	if option == nil {
		return
	}

	gs := bot.State.Guild(true, strconv.FormatInt(menu.GuildID, 10))
	gs.RLock()
	name := gs.Guild.Name
	gs.RUnlock()

	resp, err := menu.MemberChooseOption(ra, gs, option)
	if err != nil {
		logrus.WithError(err).WithField("guild", menu.GuildID).Error("Failed applying role from menu")
	}
	if resp != "" {
		bot.SendDM(ra.UserID, "**"+name+"**: "+resp)
	}
}

func (rm *RoleMenu) MemberChooseOption(ra *discordgo.MessageReactionAdd, gs *dstate.GuildState, option *RoleMenuOption) (resp string, err error) {
	cmdId, err := option.Value("role_command_id")
	if err != nil {
		return "An error occured giving you the role", err
	}

	// Not very pretty, but a limitation of kallax atm...
	cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByID(int64(*(cmdId.(*kallax.NumericID)))).WithGroup())
	if err != nil {
		return "An error occured giving you the role", err
	}

	member, err := bot.GetMember(gs.ID(), ra.UserID)
	if err != nil {
		return "An error occured giving you the role", err
	}

	given, err := AssignRole(rm.GuildID, member, cmd)
	if err != nil {
		return HumanizeAssignError(gs, err)
	}

	if given {
		return "Gave you the role!", nil
	}

	return "Took away your role!", nil
}

func RoleMenuOptionLessFunc(slice []*RoleMenuOption) func(int, int) bool {
	return func(i, j int) bool {
		// Compare timestamps if positions are equal, for deterministic output
		if slice[i].RoleCmd.Position == slice[0].RoleCmd.Position {
			return slice[i].RoleCmd.CreatedAt.After(slice[j].RoleCmd.CreatedAt)
		}

		if slice[i].RoleCmd.Position > slice[j].RoleCmd.Position {
			return false
		}

		return true
	}
}

func handleMessageRemove(evt *eventsystem.EventData) {
	if evt.MessageDelete != nil {
		messageRemoved(evt.MessageDelete.Message.ID)
	} else if evt.MessageDeleteBulk != nil {
		for _, v := range evt.MessageDeleteBulk.Messages {
			messageRemoved(v)
		}
	}
}

func messageRemoved(id string) {
	result, err := roleMenuStore.RawExec("DELETE FROM role_menus WHERE message_id=$1", id)
	if err != nil {
		logrus.WithError(err).Error("Failed removing old role menus")
	}

	if result > 0 {
		logrus.Infof("Deleetd %d role menus", result)
	}
}
