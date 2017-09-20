package rolecommands

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"gopkg.in/src-d/go-kallax.v1"
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

	resp, err := model.NextSetupStep()

	content := msg.Content + "\n" + resp
	_, err = common.BotSession.ChannelMessageEdit(parsed.Channel.ID(), msg.ID, content)
	return "", err
}

func (rm *RoleMenu) NextSetupStep() (resp string, err error) {
	commands, err := cmdStore.FindAll(NewRoleCommandQuery().FindByGroup(rm.Group.ID).Order(kallax.Desc(Schema.RoleCommand.ID)))
	if err != nil {
		return "Failed fetching commands for role group", err
	}

	var nextCmd *RoleCommand

OUTER:
	for _, cmd := range commands {
		for _, option := range rm.Options {
			if cmd.ID == option.RoleCmd.ID {
				continue OUTER
			}
		}

		// New command is cmd
		nextCmd = cmd
	}

	if nextCmd == nil {
		rm.State = RoleMenuStateDone
		roleMenuStore.Update(rm, Schema.RoleMenu.State)
		return "Done setting up!", nil
	}

	rm.NextRoleCommand = nextCmd
	rm.Options = nil
	roleMenuStore.Debug().Update(rm, Schema.RoleMenu.NextRoleCommandFK)

	return "Continue setup: React with the emoji for the role command: `" + nextCmd.Name + "` on the original message", nil
}

func (rm *RoleMenu) UpdateMenuMessage() error {
	newMsg := "Role Menu\n\n"
	for k, v := range rm.Options {
		// Fetch the roel command if missing
		if v.RoleCmd == nil {
			cmdId, err := v.Value("role_command_id")
			if err != nil {
				return err
			}
			cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByID(cmdId.(int64)))
			if err != nil {
				return err
			}
			v.RoleCmd = cmd
		}
		logrus.Println(k, v.RoleCmd)
		emoji := v.UnicodeEmoji
		if v.EmojiID != 0 {
			emoji = fmt.Sprintf("<:a:%d>", v.EmojiID)
		}
		newMsg += fmt.Sprintf("%s: `%s`\n", emoji, v.RoleCmd.Name)
	}

	_, err := common.BotSession.ChannelMessageEdit(strconv.FormatInt(rm.ChannelID, 10), strconv.FormatInt(rm.MessageID, 10), newMsg)
	return err
}

func (rm *RoleMenu) ContinueSetup(ra *discordgo.MessageReactionAdd) (resp string, err error) {
	if common.MustParseInt(ra.UserID) != rm.OwnerID {
		return "You're not the owner of this menu", nil
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

	logrus.Info("NexT: ", rm.NextRoleCommand)
	model := &RoleMenuOption{
		Menu:    rm,
		RoleCmd: rm.NextRoleCommand,
		EmojiID: parsedID,
	}

	if ra.Emoji.ID == "" {
		model.UnicodeEmoji = ra.Emoji.Name
	}

	err = roleMenuOptionStore.DebugWith(kallaxDebugger).Insert(model)
	if err != nil {
		return "Failed inserting option", err
	}

	rm.Options = append(rm.Options, model)
	err = rm.UpdateMenuMessage()
	if err != nil {
		return "Failed updating", err
	}

	return rm.NextSetupStep()
}

func handleReactionAdd(evt *eventsystem.EventData) {
	ra := evt.MessageReactionAdd

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

	logrus.Println(option)
	logrus.Println(option.RoleCmd)
	logrus.Println(option.VirtualColumn("role_cmd_id"))
}
