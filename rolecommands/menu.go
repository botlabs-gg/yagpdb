package rolecommands

import (
	"database/sql"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/rolecommands/models"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/volatiletech/null.v6"
	"sort"
	"strconv"
)

func CmdFuncRoleMenu(parsed *dcmd.Data) (interface{}, error) {
	member, err := bot.GetMember(parsed.GS.ID(), parsed.Msg.Author.ID)
	if err != nil {
		return "Failed retrieving member", err
	}

	ok, err := bot.AdminOrPerm(discordgo.PermissionManageServer, member.User.ID, parsed.CS.ID())
	if err != nil {
		return "Failed checkign your perms", err
	}

	if !ok {
		return "You do not have the proper permissions (Manage Server) to create a role menu", nil
	}

	group, err := models.RoleGroupsG(qm.Where("guild_id=?", parsed.GS.ID()), qm.Where("name ILIKE ?", parsed.Args[0].Str())).One()
	if err != nil {
		if err == sql.ErrNoRows {
			return "Did not find the role command group specified, make sure you typed it right", nil
		}

		return "Failed retrieving the group", err
	}

	if c, _ := models.RoleCommandsG(qm.Where("role_group_id=?", group.ID)).Count(); c < 1 {
		return "No commands in group, set them up in the control panel at: <https://yagpdb.xyz/manage>", nil
	}

	// set up the message if not provided
	msg, err := common.BotSession.ChannelMessageSend(parsed.CS.ID(), "Role menu\nSetting up...")
	if err != nil {
		_, dErr := common.DiscordError(err)
		errStr := "Failed creating the menu message, check the permissions on the channel"
		if dErr != "" {
			errStr += ", discord responded with: " + errStr
		}
		return errStr, err
	}

	model := &models.RoleMenu{
		MessageID: common.MustParseInt(msg.ID),
		GuildID:   common.MustParseInt(parsed.GS.ID()),
		OwnerID:   common.MustParseInt(parsed.Msg.Author.ID),
		ChannelID: common.MustParseInt(parsed.Msg.ChannelID),

		OwnMessage:  true,
		RoleGroupID: null.Int64From(group.ID),
	}

	err = model.InsertG()
	if err != nil {
		return "Failed setting up menu", err
	}

	resp, err := NextRoleMenuSetupStep(model, nil, true)

	content := msg.Content + "\n" + resp
	_, err = common.BotSession.ChannelMessageEdit(parsed.CS.ID(), msg.ID, content)
	return "", err
}

func NextRoleMenuSetupStep(rm *models.RoleMenu, opts []*models.RoleMenuOption, first bool) (resp string, err error) {
	commands, err := models.RoleCommandsG(qm.Where("role_group_id = ?", rm.RoleGroupID)).All()
	if err != nil {
		return "Failed fetching commands for role group", err
	}

	var nextCmd *models.RoleCommand

	sort.Slice(commands, RoleCommandsLessFunc(commands))

	if first {
		if len(commands) > 0 {
			nextCmd = commands[0]
		}
	} else {
		// Find next command, making sure we dont do any duplicate ones

	OUTER:
		for _, cmd := range commands {
			for _, option := range opts {
				if cmd.ID == option.RoleCommandID.Int64 {
					continue OUTER
				}
			}

			// New command is cmd
			nextCmd = cmd
			break
		}
	}

	if nextCmd == nil {
		rm.State = RoleMenuStateDone
		rm.UpdateG()
		return "Done setting up!", nil
	}

	rm.NextRoleCommandID = null.Int64From(nextCmd.ID)
	rm.UpdateG(models.RoleMenuColumns.NextRoleCommandID)
	if first {
		return "**Start setup**: React with the emoji for the role command: `" + nextCmd.Name + "` on the this message\nNote: the bot has to be on the server where the emoji is from, otherwise it wont be able to use it", nil
	}
	return "**Continue setup**: React with the emoji for the role command: `" + nextCmd.Name + "` on the original message\nNote: the bot has to be on the server where the emoji is from, otherwise it wont be able to use it", nil
}

func UpdateRoleMenuMessage(rm *models.RoleMenu, opts []*models.RoleMenuOption) error {
	newMsg := "**Role Menu**: react to give yourself a role\n\n"

	pairs := make([]*OptionCommandPair, 0, len(opts))
	for _, v := range opts {
		cmd, err := v.RoleCommandG().One()
		if err != nil {
			return err
		}

		pairs = append(pairs, &OptionCommandPair{Command: cmd, Option: v})
	}

	sort.Slice(pairs, OptionCommandPairLessFunc(pairs))

	for _, option := range opts {

		cmd, err := option.RoleCommandG().One()
		if err != nil {
			return err
		}

		emoji := option.UnicodeEmoji
		if option.EmojiID != 0 {
			emoji = fmt.Sprintf("<:yagpdb:%d>", option.EmojiID)
		}
		newMsg += fmt.Sprintf("%s : `%s`\n\n", emoji, cmd.Name)
	}

	_, err := common.BotSession.ChannelMessageEdit(strconv.FormatInt(rm.ChannelID, 10), strconv.FormatInt(rm.MessageID, 10), newMsg)
	return err
}

func ContinueRoleMenuSetup(rm *models.RoleMenu, ra *discordgo.MessageReactionAdd) (resp string, err error) {
	if common.MustParseInt(ra.UserID) != rm.OwnerID {
		common.BotSession.MessageReactionRemove(ra.ChannelID, ra.MessageID, ra.Emoji.APIName(), ra.UserID)
		return "This menu is still being set up, wait until the owner of this menu is done.", nil
	}

	parsedID := int64(0)
	if ra.Emoji.ID != "" {
		parsedID = common.MustParseInt(ra.Emoji.ID)
	}

	currentOpts, err := rm.RoleMenuOptionsG().All()
	if err != nil && err != sql.ErrNoRows {
		return "Error communicating with DB", err
	}

	// Make sure this emoji isnt used to another option
	for _, option := range currentOpts {
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

	model := &models.RoleMenuOption{
		RoleMenuID:    rm.MessageID,
		RoleCommandID: rm.NextRoleCommandID,
		EmojiID:       parsedID,
	}

	if ra.Emoji.ID == "" {
		model.UnicodeEmoji = ra.Emoji.Name
	}

	err = model.InsertG()
	if err != nil {
		return "Failed inserting option", err
	}

	err = common.BotSession.MessageReactionAdd(ra.ChannelID, ra.MessageID, ra.Emoji.APIName())
	if err != nil {
		logrus.WithError(err).WithField("emoji", ra.Emoji.APIName()).Error("Failed reacting")
	}

	currentOpts = append(currentOpts, model)
	// rm.Options = append(rm.Options, model)
	// err = rm.UpdateMenuMessage()
	err = UpdateRoleMenuMessage(rm, currentOpts)
	if err != nil {
		return "Failed updating", err
	}

	// return rm.NextSetupStep(false)
	return NextRoleMenuSetupStep(rm, currentOpts, false)
}

type OptionCommandPair struct {
	Option  *models.RoleMenuOption
	Command *models.RoleCommand
}

func handleReactionAdd(evt *eventsystem.EventData) {
	ra := evt.MessageReactionAdd
	if ra.UserID == common.BotUser.ID {
		return
	}

	menu, err := models.FindRoleMenuG(common.MustParseInt(ra.MessageID))
	// menu, err := .FindByMessageID(common.MustParseInt(ra.MessageID)).WithOptions(nil).WithNextRoleCommand().WithGroup())
	if err != nil {
		if err != sql.ErrNoRows {
			logrus.WithError(err).Error("RoleCommnadsMenu: Failed finding menu")
		}
		return
	}

	if menu.State == RoleMenuStateSettingUp {
		resp, err := ContinueRoleMenuSetup(menu, ra)
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

	opts, err := menu.RoleMenuOptionsG().All()
	if err != nil && err != sql.ErrNoRows {
		logrus.WithError(err).Error("Failed retrieving rolemenu options")
		return
	}

	var option *models.RoleMenuOption
	if ra.Emoji.ID != "" {
		// This is a custom emoji
		parsedID := common.MustParseInt(ra.Emoji.ID)
		for _, v := range opts {
			if v.EmojiID != 0 && v.EmojiID == parsedID {
				option = v
			}
		}
	} else {
		// Unicode emoji
		for _, v := range opts {
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

	resp, err := MemberChooseOption(menu, ra, gs, option)
	if err != nil {
		logrus.WithError(err).WithField("guild", menu.GuildID).Error("Failed applying role from menu")
	}
	if resp != "" {
		bot.SendDM(ra.UserID, "**"+name+"**: "+resp)
	}
}

func MemberChooseOption(rm *models.RoleMenu, ra *discordgo.MessageReactionAdd, gs *dstate.GuildState, option *models.RoleMenuOption) (resp string, err error) {
	cmd, err := option.RoleCommandG().One()
	if err != nil {
		return "An error occured giving you the role", err
	}

	pair := &CommandGroupPair{Command: cmd}
	if cmd.RoleGroupID.Valid {
		pair.Group, err = cmd.RoleGroupG().One()
		if err != nil {
			return "An error occured giving you the role", err
		}
	}

	member, err := bot.GetMember(gs.ID(), ra.UserID)
	if err != nil {
		return "An error occured giving you the role", err
	}

	given, err := AssignRole(rm.GuildID, member, pair)
	if err != nil {
		return HumanizeAssignError(gs, err)
	}

	if given {
		return "Gave you the role!", nil
	}

	return "Took away your role!", nil
}

func OptionCommandPairLessFunc(slice []*OptionCommandPair) func(int, int) bool {
	return func(i, j int) bool {
		// Compare timestamps if positions are equal, for deterministic output
		if slice[i].Command.Position == slice[0].Command.Position {
			return slice[i].Command.CreatedAt.After(slice[j].Command.CreatedAt)
		}

		if slice[i].Command.Position > slice[j].Command.Position {
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
	err := models.RoleMenusG(qm.Where("message_id=?", id)).DeleteAll()
	if err != nil {
		logrus.WithError(err).Error("Failed removing old role menus")
	}
}
