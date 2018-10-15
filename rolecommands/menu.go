package rolecommands

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/rolecommands/models"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"sort"
)

func cmdFuncRoleMenuCreate(parsed *dcmd.Data) (interface{}, error) {
	logrus.Println("Weeew")

	group, err := models.RoleGroups(qm.Where("guild_id=?", parsed.GS.ID), qm.Where("name ILIKE ?", parsed.Args[0].Str()), qm.Load("RoleCommands")).OneG(parsed.Context())
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return "Did not find the role command group specified, make sure you typed it right, if you haven't set one up yet you can do so in the control panel.", nil
		}

		return nil, err
	}

	cmdsLen := len(group.R.RoleCommands)
	if cmdsLen > 20 || cmdsLen < 1 {
		if cmdsLen < 1 {
			return "No commands in group, set them up in the control panel.", nil
		} else {
			return "Can't do this, that group has over 20 roles in it, but there can only be 20 reactions on 1 message.", nil
		}
	}

	model := &models.RoleMenu{
		GuildID:   parsed.GS.ID,
		OwnerID:   parsed.Msg.Author.ID,
		ChannelID: parsed.Msg.ChannelID,

		RoleGroupID:                null.Int64From(group.ID),
		OwnMessage:                 true,
		DisableSendDM:              parsed.Switches["nodm"].Value != nil && parsed.Switches["nodm"].Value.(bool),
		RemoveRoleOnReactionRemove: true,
	}

	if group != nil {
		model.RoleGroupID = null.Int64From(group.ID)
	}

	var msg *discordgo.Message
	if parsed.Switches["m"].Value != nil {
		model.OwnMessage = false

		id := parsed.Switches["m"].Int64()
		msg, err = common.BotSession.ChannelMessage(parsed.CS.ID, id)
		if err != nil {
			return nil, err
		}

		model.MessageID = id
	} else {

		// set up the message if not provided
		msg, err = common.BotSession.ChannelMessageSend(parsed.CS.ID, "Role menu\nSetting up...")
		if err != nil {
			_, dErr := common.DiscordError(err)
			errStr := "Failed creating the menu message, check the permissions on the channel"
			if dErr != "" {
				errStr += ", discord responded with: " + errStr
			}
			return errStr, err
		}

		model.MessageID = msg.ID
	}

	err = model.InsertG(parsed.Context(), boil.Infer())
	if err != nil {
		return "Failed setting up menu", err
	}

	model.R = model.R.NewStruct()
	model.R.RoleGroup = group

	resp, err := NextRoleMenuSetupStep(parsed.Context(), model, true)

	if model.OwnMessage {
		content := msg.Content + "\n" + resp
		_, err = common.BotSession.ChannelMessageEdit(parsed.CS.ID, msg.ID, content)
		return "", err
	}

	return resp, err
}

func cmdFuncRoleMenuUpdate(parsed *dcmd.Data) (interface{}, error) {
	mID := parsed.Args[0].Int64()
	menu, err := FindRolemenuFull(parsed.Context(), mID)
	if err != nil {
		return "Couldn't find menu", nil
	}

	return UpdateMenu(parsed, menu)
}

func UpdateMenu(parsed *dcmd.Data, menu *models.RoleMenu) (interface{}, error) {
	if menu.State == RoleMenuStateSettingUp {
		return "Already setting this menu up", nil
	}

	menu.State = RoleMenuStateSettingUp

	if parsed.Switches["nodm"].Value != nil && parsed.Switches["nodm"].Value.(bool) {
		menu.DisableSendDM = !menu.DisableSendDM
	}

	if parsed.Switches["rr"].Value != nil && parsed.Switches["rr"].Value.(bool) {
		menu.RemoveRoleOnReactionRemove = !menu.RemoveRoleOnReactionRemove
	}

	menu.UpdateG(parsed.Context(), boil.Infer())

	if menu.OwnMessage {
		UpdateRoleMenuMessage(parsed.Context(), menu)
	}

	// Add all mising options
	return NextRoleMenuSetupStep(parsed.Context(), menu, false)
}

func NextRoleMenuSetupStep(ctx context.Context, rm *models.RoleMenu, first bool) (resp string, err error) {
	var nextCmd *models.RoleCommand

	commands := rm.R.RoleGroup.R.RoleCommands
	sort.Slice(commands, RoleCommandsLessFunc(commands))

	if first {
		if len(commands) > 0 {
			nextCmd = commands[0]
		}
	} else {
		// Find next command, making sure we dont do any duplicate ones

	OUTER:
		for _, cmd := range commands {
			for _, option := range rm.R.RoleMenuOptions {
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
		rm.UpdateG(ctx, boil.Infer())

		nodmFlagHelp := fmt.Sprintf("`-nodm: %t` toggle with `rolemenu update -nodm %d`: disables dm messages.", rm.DisableSendDM, rm.MessageID)
		rrFlagHelp := fmt.Sprintf("`-rr: %t` toggle with `rolemenu update -rr %d`: removing reactions removes the role.", rm.RemoveRoleOnReactionRemove, rm.MessageID)
		return fmt.Sprintf("Done setting up! you can delete all the messages now (except for the menu itself)\n\nFlags:\n%s\n%s", nodmFlagHelp, rrFlagHelp), nil
	}

	rm.NextRoleCommandID = null.Int64From(nextCmd.ID)
	rm.UpdateG(ctx, boil.Whitelist(models.RoleMenuColumns.NextRoleCommandID))
	if first && rm.OwnMessage {
		return "**Rolemenu Setup**: React with the emoji for the role command: `" + nextCmd.Name + "` on this message\nNote: the bot has to be on the server where the emoji is from, otherwise it won't be able to use it", nil
	}
	return "**Rolemenu Setup**: React with the emoji for the role command: `" + nextCmd.Name + "` on the **Menu message (not this one)**\nNote: the bot has to be on the server where the emoji is from, otherwise it won't be able to use it", nil
}

func UpdateRoleMenuMessage(ctx context.Context, rm *models.RoleMenu) error {
	newMsg := "**Role Menu**: react to give yourself a role\n\n"

	opts := rm.R.RoleMenuOptions
	sort.Slice(opts, OptionsLessFunc(opts))

	for _, opt := range opts {
		cmd := opt.R.RoleCommand

		emoji := opt.UnicodeEmoji
		if opt.EmojiID != 0 {
			emoji = fmt.Sprintf("<:yagpdb:%d>", opt.EmojiID)
		}

		newMsg += fmt.Sprintf("%s : `%s`\n\n", emoji, cmd.Name)
	}

	_, err := common.BotSession.ChannelMessageEdit(rm.ChannelID, rm.MessageID, newMsg)
	return err
}

func ContinueRoleMenuSetup(ctx context.Context, rm *models.RoleMenu, emoji *discordgo.Emoji, userID int64) (resp string, err error) {
	if userID != rm.OwnerID {
		common.BotSession.MessageReactionRemove(rm.ChannelID, rm.MessageID, emoji.APIName(), userID)
		return "This menu is still being set up, wait until the owner of this menu is done.", nil
	}

	currentOpts := rm.R.RoleMenuOptions

	// Make sure this emoji isnt used to another option
	for _, option := range currentOpts {
		if emoji.ID != 0 {
			if emoji.ID == option.EmojiID && option.EmojiID != 0 {
				return "Emoji already used for another option", nil
			}
		} else {
			if emoji.Name == option.UnicodeEmoji && option.UnicodeEmoji != "" {
				return "Emoji already used for another option", nil
			}
		}
	}

	model := &models.RoleMenuOption{
		RoleMenuID:    rm.MessageID,
		RoleCommandID: rm.NextRoleCommandID,
		EmojiID:       emoji.ID,
	}

	if emoji.ID == 0 {
		model.UnicodeEmoji = emoji.Name
	}

	err = model.InsertG(ctx, boil.Infer())
	if err != nil {
		return "Failed inserting option", err
	}

	err = common.BotSession.MessageReactionAdd(rm.ChannelID, rm.MessageID, emoji.APIName())
	if err != nil {
		logrus.WithError(err).WithField("emoji", emoji.APIName()).Error("Failed reacting")
	}

	model.R = model.R.NewStruct()
	for _, cmd := range rm.R.RoleGroup.R.RoleCommands {
		if cmd.ID == model.RoleCommandID.Int64 {
			model.R.RoleCommand = cmd
			break
		}
	}

	rm.R.RoleMenuOptions = append(rm.R.RoleMenuOptions, model)

	if rm.OwnMessage {
		err = UpdateRoleMenuMessage(ctx, rm)
		if err != nil {
			return "Failed updating message", err
		}
	}

	// return rm.NextSetupStep(false)
	return NextRoleMenuSetupStep(ctx, rm, false)
}

func getReactionDetails(evt *eventsystem.EventData) (emoji *discordgo.Emoji, cID, gID, uID, mID int64, add bool) {
	if evt.Type == eventsystem.EventMessageReactionAdd {
		ra := evt.MessageReactionAdd()
		cID = ra.ChannelID
		uID = ra.UserID
		gID = ra.GuildID
		mID = ra.MessageID
		emoji = &ra.Emoji
		add = true
	} else {
		rr := evt.MessageReactionRemove()
		cID = rr.ChannelID
		uID = rr.UserID
		gID = rr.GuildID
		mID = rr.MessageID
		emoji = &rr.Emoji
	}

	return
}

func findOptionFromEmoji(emoji *discordgo.Emoji, opts []*models.RoleMenuOption) *models.RoleMenuOption {
	if emoji.ID != 0 {
		// This is a custom emoji
		for _, v := range opts {
			if v.EmojiID != 0 && v.EmojiID == emoji.ID {
				return v
			}
		}
	} else {
		// Unicode emoji
		for _, v := range opts {
			if v.UnicodeEmoji == emoji.Name && v.EmojiID == 0 {
				return v
			}
		}
	}

	return nil
}

func handleReactionAddRemove(evt *eventsystem.EventData) {
	emoji, cID, _, uID, mID, raAdd := getReactionDetails(evt)
	if uID == common.BotUser.ID {
		return
	}

	menu, err := FindRolemenuFull(evt.Context(), mID)
	if err != nil {
		if err != sql.ErrNoRows {
			logrus.WithError(err).Error("RoleCommandsMenu: Failed finding menu")
		}
		return
	}

	// Continue setup if were doing that
	if menu.State == RoleMenuStateSettingUp {
		if !raAdd {
			// ignore reaction removes in this state
			return
		}

		resp, err := ContinueRoleMenuSetup(evt.Context(), menu, emoji, uID)
		if err != nil {
			logrus.WithError(err).Error("RoleCommandsMenu: Failed continuing role menu setup")
		}

		if resp != "" {
			_, err = common.BotSession.ChannelMessageSend(cID, "Role menu setup: "+resp)
			if err != nil {
				logrus.WithError(err).Error("RoleCommandsMenu: Failed sending new response")
			}
		}

		return
	}

	if !menu.RemoveRoleOnReactionRemove && !raAdd {
		return // only go further is this flag is enabled
	}

	// Find the option model from the reaction
	option := findOptionFromEmoji(emoji, menu.R.RoleMenuOptions)
	if option == nil {
		return
	}

	gs := bot.State.Guild(true, menu.GuildID)
	gs.RLock()
	name := gs.Guild.Name
	gs.RUnlock()

	resp, err := MemberChooseOption(evt.Context(), menu, gs, option, uID, emoji, raAdd)
	if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingPermissions) {
		logrus.WithError(err).WithField("option", option.ID).WithField("guild", menu.GuildID).Error("Failed applying role from menu")
	}

	if resp != "" {
		bot.SendDM(uID, "**"+name+"**: "+resp)
	}
}

func MemberChooseOption(ctx context.Context, rm *models.RoleMenu, gs *dstate.GuildState, option *models.RoleMenuOption, userID int64, emoji *discordgo.Emoji, raAdd bool) (resp string, err error) {
	cmd := option.R.RoleCommand

	member, err := bot.GetMember(gs.ID, userID)
	if err != nil {
		return "An error occured giving you the role", err
	}

	if member.Bot {
		// ignore bots
		return "", nil
	}

	if rm.RemoveRoleOnReactionRemove {
		//  Strictly assign or remove based on wether the reaction was added or removed
		if raAdd {
			var added bool
			added, err = AssignRole(ctx, member, cmd)
			if err == nil && added {
				resp = "Gave you the role!"
			}
		} else {
			var removed bool
			removed, err = RemoveRole(ctx, member, cmd)
			if err == nil && removed {
				resp = "Took away the role!"
			}
		}
	} else {
		// Just toggle...
		var given bool
		given, err = CheckToggleRole(ctx, member, cmd)
		if err == nil {
			if given {
				resp = "Gave you the role!"
			} else {
				resp = "Took away the role!"
			}
		}
	}

	if rm.DisableSendDM {
		resp = ""
	}

	if err != nil {
		if raAdd {
			common.BotSession.MessageReactionRemove(rm.ChannelID, rm.MessageID, emoji.APIName(), userID)
		}
		resp, err = HumanizeAssignError(gs, err)
	}

	if resp != "" {
		resp = cmd.Name + ": " + resp
	}

	return
}

func OptionsLessFunc(slice []*models.RoleMenuOption) func(int, int) bool {
	return func(i, j int) bool {
		// Compare timestamps if positions are equal, for deterministic output
		if slice[i].R.RoleCommand.Position == slice[j].R.RoleCommand.Position {
			return slice[i].R.RoleCommand.CreatedAt.After(slice[j].R.RoleCommand.CreatedAt)
		}

		if slice[i].R.RoleCommand.Position > slice[j].R.RoleCommand.Position {
			return false
		}

		return true
	}
}

func handleMessageRemove(evt *eventsystem.EventData) {
	if evt.Type == eventsystem.EventMessageDelete {
		messageRemoved(evt.Context(), evt.MessageDelete().Message.ID)
	} else if evt.Type == eventsystem.EventMessageDeleteBulk {
		for _, v := range evt.MessageDeleteBulk().Messages {
			messageRemoved(evt.Context(), v)
		}
	}
}

func messageRemoved(ctx context.Context, id int64) {
	_, err := models.RoleMenus(qm.Where("message_id=?", id)).DeleteAll(ctx, common.PQ)
	if err != nil {
		logrus.WithError(err).Error("Failed removing old role menus")
	}
}

func FindRolemenuFull(ctx context.Context, mID int64) (*models.RoleMenu, error) {
	return models.RoleMenus(qm.Where("message_id = ?", mID), qm.Load("RoleMenuOptions.RoleCommand"), qm.Load("RoleGroup.RoleCommands")).OneG(ctx)
}

func cmdFuncRoleMenuResetReactions(data *dcmd.Data) (interface{}, error) {
	mID := data.Args[0].Int64()
	menu, err := FindRolemenuFull(data.Context(), mID)
	if err != nil {
		return "Couldn't find menu", nil
	}

	err = common.BotSession.MessageReactionsRemoveAll(menu.ChannelID, menu.MessageID)
	if err != nil {
		return nil, err
	}

	sort.Slice(menu.R.RoleMenuOptions, OptionsLessFunc(menu.R.RoleMenuOptions))

	for _, option := range menu.R.RoleMenuOptions {
		emoji := option.UnicodeEmoji
		if option.EmojiID != 0 {
			emoji = "aaa:" + discordgo.StrID(option.EmojiID)
		}

		logrus.Println(emoji)
		err := common.BotSession.MessageReactionAdd(menu.ChannelID, menu.MessageID, emoji)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
