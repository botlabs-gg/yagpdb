package rolecommands

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/rolecommands/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func cmdFuncRoleMenuCreate(parsed *dcmd.Data) (interface{}, error) {
	group, err := models.RoleGroups(qm.Where("guild_id=?", parsed.GS.ID), qm.Where("name ILIKE ?", parsed.Args[0].Str()), qm.Load("RoleCommands")).OneG(parsed.Context())
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return "Did not find the role command group specified, make sure you typed it right, if you haven't set one up yet you can do so in the control panel.", nil
		}

		return nil, err
	}

	skipAmount := parsed.Switches["skip"].Int()

	cmdsLen := len(group.R.RoleCommands)
	if cmdsLen < 1 {
		return "No commands in this group, set them up in the control panel.", nil
	}

	model := &models.RoleMenu{
		GuildID:   parsed.GS.ID,
		OwnerID:   parsed.Msg.Author.ID,
		ChannelID: parsed.Msg.ChannelID,

		RoleGroupID:                null.Int64From(group.ID),
		OwnMessage:                 true,
		DisableSendDM:              parsed.Switches["nodm"].Value != nil && parsed.Switches["nodm"].Value.(bool),
		RemoveRoleOnReactionRemove: true,
		SkipAmount:                 skipAmount,
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
				errStr += ", Discord responded with: " + errStr
			}
			return errStr, err
		}

		model.MessageID = msg.ID
	}

	err = model.InsertG(parsed.Context(), boil.Infer())
	if err != nil {
		if common.ErrPQIsUniqueViolation(err) {
			return "There is already a menu on that message, use `rolemenu update ...` to update it", nil
		}

		return "Failed setting up menu", err
	}

	model.R = model.R.NewStruct()
	model.R.RoleGroup = group

	resp, err := NextRoleMenuSetupStep(parsed.Context(), model, true)
	updateSetupMessage(parsed.Context(), model, resp)
	return nil, err
}

func cmdFuncRoleMenuUpdate(parsed *dcmd.Data) (interface{}, error) {
	mID := parsed.Args[0].Int64()
	menu, err := FindRolemenuFull(parsed.Context(), mID, parsed.GS.ID)
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

	// don't reuse the old setup message
	menu.SetupMSGID = 0
	menu.OwnerID = parsed.Msg.Author.ID

	menu.UpdateG(parsed.Context(), boil.Infer())

	if menu.OwnMessage {
		UpdateRoleMenuMessage(parsed.Context(), menu)
	}

	// Add all mising options
	resp, err := NextRoleMenuSetupStep(parsed.Context(), menu, false)
	if resp != "" {
		createSetupMessage(parsed.Context(), menu, resp, true)
	}
	return nil, err
}

func NextRoleMenuSetupStep(ctx context.Context, rm *models.RoleMenu, first bool) (resp string, err error) {

	commands := rm.R.RoleGroup.R.RoleCommands
	sort.Slice(commands, RoleCommandsLessFunc(commands))

	var nextCmd *models.RoleCommand
	numDone := 0

OUTER:
	for i, cmd := range commands {
		if i < rm.SkipAmount {
			continue
		}

		for _, option := range rm.R.RoleMenuOptions {
			if cmd.ID == option.RoleCommandID.Int64 {
				numDone++
				continue OUTER
			}
		}

		// New command is cmd
		if nextCmd == nil {
			nextCmd = cmd
		}
	}

	if nextCmd == nil || len(rm.R.RoleMenuOptions) >= 20 {
		extra := ""
		if len(rm.R.RoleMenuOptions) >= 20 && nextCmd != nil {
			extra = fmt.Sprintf("\n\nMessages can contain max 20 reactions, couldn't fit them all into this one, you can add the remaining to another menu using `rolemenu create %s -skip %d`", rm.R.RoleGroup.Name, rm.SkipAmount+20)
			rm.FixedAmount = true
		}

		rm.State = RoleMenuStateDone
		rm.UpdateG(ctx, boil.Infer())

		flagHelp := StrFlags(rm)
		return fmt.Sprintf("Done setting up! You can delete all the messages now (except for the menu itself)\n\nFlags:\n%s%s", flagHelp, extra), nil
	}

	rm.NextRoleCommandID = null.Int64From(nextCmd.ID)
	rm.UpdateG(ctx, boil.Whitelist(models.RoleMenuColumns.NextRoleCommandID))

	totalCommands := len(commands) - rm.SkipAmount
	resp = fmt.Sprintf("[%d/%d]\n", numDone, totalCommands)

	return resp + "React with the emoji for the role command: `" + nextCmd.Name + "`\nNote: The bot has to be on the server where the emoji is from, otherwise it won't be able to use it", nil
}

func StrFlags(rm *models.RoleMenu) string {
	nodmFlagHelp := fmt.Sprintf("`-nodm: %t` toggle with `rolemenu update -nodm %d`: disables dm messages.", rm.DisableSendDM, rm.MessageID)
	rrFlagHelp := fmt.Sprintf("`-rr: %t` toggle with `rolemenu update -rr %d`: removing reactions removes the role.", rm.RemoveRoleOnReactionRemove, rm.MessageID)
	return nodmFlagHelp + "\n" + rrFlagHelp
}

func UpdateRoleMenuMessage(ctx context.Context, rm *models.RoleMenu) error {
	newMsg := "**Role Menu: " + rm.R.RoleGroup.Name + "**\nReact to give yourself a role.\n\n"

	opts := rm.R.RoleMenuOptions
	sort.Slice(opts, OptionsLessFunc(opts))

	for _, opt := range opts {
		cmd := opt.R.RoleCommand

		emoji := opt.UnicodeEmoji
		if opt.EmojiID != 0 {
			if opt.EmojiAnimated {
				emoji = fmt.Sprintf("<a:yagpdb:%d>", opt.EmojiID)
			} else {
				emoji = fmt.Sprintf("<:yagpdb:%d>", opt.EmojiID)
			}
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

	// next role command id can be null if the relevant role command was deleted during setup
	if rm.NextRoleCommandID.Valid {
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

		err = common.BotSession.MessageReactionAdd(rm.ChannelID, rm.MessageID, emoji.APIName())
		if err != nil {
			code, _ := common.DiscordError(err)
			switch code {
			case discordgo.ErrCodeUnknownEmoji:
				return "I do not have access to that emoji, i can only use emojis from servers im on.", nil
			case discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions:
				return "I do not have permissions to add reactions here, please give me that permission to continue the setup.", nil
			default:
				logger.WithError(err).WithField("emoji", emoji.APIName()).Error("Failed reacting")
				return "An unknown error occured, please retry adding that emoji", nil
			}
		}

		model := &models.RoleMenuOption{
			RoleMenuID:    rm.MessageID,
			RoleCommandID: rm.NextRoleCommandID,
			EmojiID:       emoji.ID,
			EmojiAnimated: emoji.Animated,
		}

		if emoji.ID == 0 {
			model.UnicodeEmoji = emoji.Name
		}

		err = model.InsertG(ctx, boil.Infer())
		if err != nil {
			return "Failed inserting option into the database, please retry adding the emoji.", err
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
				code, _ := common.DiscordError(err)
				switch code {
				case discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions:
					return "I do not have permissions to update the menu message, please give me the proper permissions for me to update the menu message.", nil
				default:
					return "An error occured updating the menu message, use the `rolemenu update <id>` command to manually update the message", err
				}
			}
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
	emoji, _, gID, uID, mID, raAdd := getReactionDetails(evt)
	if uID == common.BotUser.ID {
		return
	}

	menu, err := FindRolemenuFull(evt.Context(), mID, gID)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.WithError(err).Error("RoleCommandsMenu: Failed finding menu")
		}
		return
	}

	// Continue setup if were doing that
	if menu.State != RoleMenuStateDone {
		if !raAdd {
			// ignore reaction removes in this state
			return
		}

		resp, err := MenuReactedNotDone(evt.Context(), menu, emoji, uID)
		if err != nil {
			logger.WithError(err).Error("RoleCommandsMenu: Failed continuing role menu setup, or editing menu")
		}

		if resp != "" {
			if err != nil {
				createSetupMessage(evt.Context(), menu, resp, false)
			} else {
				updateSetupMessage(evt.Context(), menu, resp)
			}
		}

		return
	}

	if !menu.RemoveRoleOnReactionRemove && !raAdd {
		return // only go further is this flag is enabled
	}

	if mID != menu.MessageID {
		return // reacted on the seutp message id, only allow setup actions there
	}

	// Find the option model from the reaction
	option := findOptionFromEmoji(emoji, menu.R.RoleMenuOptions)
	if option == nil {
		return
	}

	gs := evt.GS
	gs.RLock()
	name := gs.Guild.Name
	gs.RUnlock()

	resp, err := MemberChooseOption(evt.Context(), menu, gs, option, uID, emoji, raAdd)
	if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingPermissions) {
		logger.WithError(err).WithField("option", option.ID).WithField("guild", menu.GuildID).Error("Failed applying role from menu")
	}

	if resp != "" {
		bot.SendDM(uID, "**"+name+"**: "+resp)
	}
}

func MemberChooseOption(ctx context.Context, rm *models.RoleMenu, gs *dstate.GuildState, option *models.RoleMenuOption, userID int64, emoji *discordgo.Emoji, raAdd bool) (resp string, err error) {
	cmd := option.R.RoleCommand
	cmd.R.RoleGroup = rm.R.RoleGroup

	member, err := bot.GetMember(gs.ID, userID)
	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeUnknownMember) {
			return "", nil
		}

		return "An error occured giving you the role", err
	}

	if member.Bot {
		// ignore bots
		return "", nil
	}

	var given bool

	if rm.RemoveRoleOnReactionRemove {
		//  Strictly assign or remove based on wether the reaction was added or removed
		if raAdd {
			given, err = AssignRole(ctx, member, cmd)
			if err == nil && given {
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
		given, err = CheckToggleRole(ctx, member, cmd)
		if err == nil {
			if given {
				resp = "Gave you the role!"
			} else {
				resp = "Took away the role!"
			}
		}
	}

	go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "user_interacted_menu")

	if rm.DisableSendDM {
		resp = ""
	}

	if err != nil {
		if raAdd {
			common.BotSession.MessageReactionRemove(rm.ChannelID, rm.MessageID, emoji.APIName(), userID)
		}
		resp, err = HumanizeAssignError(gs, err)
	} else if rm.R.RoleGroup.Mode == GroupModeSingle && given {
		go removeOtherReactions(rm, option, userID)
	}

	if resp != "" {
		resp = cmd.Name + ": " + resp
	}

	return
}

// track reaction removal loop so that we can cancel them
type reactionRemovalOccurence struct {
	MessageID int64
	UserID    int64

	Stopmu sync.Mutex
	Stop   bool
}

var (
	activeReactionRemovals   = make([]*reactionRemovalOccurence, 0)
	activeReactionRemovalsmu sync.Mutex

	confDisableReactionRemovalSingleMode = config.RegisterOption("yagpdb.rolecommands.disable_reaction_removal_single_mode", "Disable reaction removal in single mode, could be heavy on number of requests", false)
)

func removeOtherReactions(rm *models.RoleMenu, option *models.RoleMenuOption, userID int64) {
	if confDisableReactionRemovalSingleMode.GetBool() {
		// since this is an experimental feature
		return
	}

	isPremium, err := premium.IsGuildPremiumCached(rm.GuildID)
	if err != nil {
		logger.WithError(err).WithField("guild", rm.GuildID).Error("Failed checking if guild is premium")
		return
	}

	if !isPremium {
		return
	}

	activeReactionRemovalsmu.Lock()
	// cancel existing ones
	for _, v := range activeReactionRemovals {
		if v.MessageID == rm.MessageID && v.UserID == userID {
			v.Stopmu.Lock()
			v.Stop = true
			v.Stopmu.Unlock()
		}
	}

	// add the new one
	cur := &reactionRemovalOccurence{
		MessageID: rm.MessageID,
		UserID:    userID,
	}
	activeReactionRemovals = append(activeReactionRemovals, cur)

	activeReactionRemovalsmu.Unlock()

	// make sure to remove it when we return
	defer func() {
		activeReactionRemovalsmu.Lock()
		for i, v := range activeReactionRemovals {
			if v == cur {
				activeReactionRemovals = append(activeReactionRemovals[:i], activeReactionRemovals[i+1:]...)
				break
			}
		}
		activeReactionRemovalsmu.Unlock()
	}()

	// actually start the reaction removal process
	for _, v := range rm.R.RoleMenuOptions {
		if v.ID == option.ID {
			continue
		}

		// check if we were cancelled
		cur.Stopmu.Lock()
		stop := cur.Stop
		cur.Stopmu.Unlock()
		if stop {
			break
		}

		emoji := v.UnicodeEmoji
		if v.EmojiID != 0 {
			emoji = "aaa:" + discordgo.StrID(v.EmojiID)
		}

		common.BotSession.MessageReactionRemove(rm.ChannelID, rm.MessageID, emoji, userID)
	}
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
		logger.WithError(err).Error("Failed removing old role menus")
	}
}

func FindRolemenuFull(ctx context.Context, mID int64, guildID int64) (*models.RoleMenu, error) {
	return models.RoleMenus(qm.Where("guild_id = ? AND (message_id = ? OR setup_msg_id = ?)", guildID, mID, mID), qm.Load("RoleMenuOptions.RoleCommand"), qm.Load("RoleGroup.RoleCommands")).OneG(ctx)
}

func cmdFuncRoleMenuResetReactions(data *dcmd.Data) (interface{}, error) {
	mID := data.Args[0].Int64()
	menu, err := FindRolemenuFull(data.Context(), mID, data.GS.ID)
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

		err := common.BotSession.MessageReactionAdd(menu.ChannelID, menu.MessageID, emoji)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func cmdFuncRoleMenuRemove(data *dcmd.Data) (interface{}, error) {
	mID := data.Args[0].Int64()
	menu, err := FindRolemenuFull(data.Context(), mID, data.GS.ID)
	if err != nil {
		return "Couldn't find menu", nil
	}

	_, err = menu.DeleteG(data.Context())
	if err != nil {
		return nil, err
	}

	return "Deleted. The bot will no longer listen for reactions on this message, you can even make another menu on it.", nil
}

func cmdFuncRoleMenuEditOption(data *dcmd.Data) (interface{}, error) {
	mID := data.Args[0].Int64()
	menu, err := FindRolemenuFull(data.Context(), mID, data.GS.ID)
	if err != nil {
		return "Couldn't find menu", nil
	}

	if menu.State != RoleMenuStateDone {
		return "This menu isn't 'done' (still being edited, or made)", nil
	}

	menu.State = RoleMenuStateEditingOptionSelecting
	menu.OwnerID = data.Msg.Author.ID
	menu.SetupMSGID = 0
	_, err = menu.UpdateG(data.Context(), boil.Whitelist("state", "owner_id", "setup_msg_id"))
	if err != nil {
		return "", err
	}

	createSetupMessage(data.Context(), menu, "React on the emoji for the option you want to change", true)
	return nil, nil
}

func cmdFuncRoleMenuComplete(data *dcmd.Data) (interface{}, error) {
	mID := data.Args[0].Int64()
	menu, err := FindRolemenuFull(data.Context(), mID, data.GS.ID)
	if err != nil {
		return "Couldn't find menu", nil
	}

	if menu.State == RoleMenuStateDone {
		return "This menu is already marked as done", nil
	}

	menu.State = RoleMenuStateDone
	menu.SetupMSGID = 0

	_, err = menu.UpdateG(data.Context(), boil.Whitelist("state", "setup_msg_id"))
	if err != nil {
		return nil, err
	}

	return "Menu marked as done", nil
}

func MenuReactedNotDone(ctx context.Context, rm *models.RoleMenu, emoji *discordgo.Emoji, userID int64) (resp string, err error) {
	if userID != rm.OwnerID {
		return "Someone is currently editing or setting up this menu, please wait", nil
	}

	switch rm.State {
	case RoleMenuStateSettingUp:
		return ContinueRoleMenuSetup(ctx, rm, emoji, userID)
	case RoleMenuStateEditingOptionSelecting:
		option := findOptionFromEmoji(emoji, rm.R.RoleMenuOptions)
		if option == nil {
			return "", nil
		}

		rm.State = RoleMenuStateEditingOptionReplacing
		rm.EditingOptionID = null.Int64From(option.ID)
		_, err := rm.UpdateG(ctx, boil.Whitelist("state", "editing_option_id"))
		if err != nil {
			return "", err
		}

		return "Editing `" + option.R.RoleCommand.Name + "`, select the new emoji for it", nil
	case RoleMenuStateEditingOptionReplacing:
		option, err := rm.EditingOption().OneG(ctx)
		if err != nil {
			// possible they might have deleted the role in the meantime, so set it to done to prevent a deadlock for this menu
			rm.State = RoleMenuStateDone
			rm.UpdateG(ctx, boil.Whitelist("state"))
			return "", err
		}

		option.EmojiID = emoji.ID
		option.EmojiAnimated = emoji.Animated
		if option.EmojiID == 0 {
			option.UnicodeEmoji = emoji.Name
		} else {
			option.UnicodeEmoji = ""
		}

		_, err = option.UpdateG(ctx, boil.Infer())
		if err != nil {
			return "", err
		}

		rm.State = RoleMenuStateDone
		rm.UpdateG(ctx, boil.Whitelist("state"))

		go common.BotSession.MessageReactionAdd(rm.ChannelID, rm.MessageID, emoji.APIName())

		if rm.OwnMessage {
			for _, v := range rm.R.RoleMenuOptions {
				if v.ID == option.ID {
					v.EmojiAnimated = option.EmojiAnimated
					v.EmojiID = option.EmojiID
					v.UnicodeEmoji = option.UnicodeEmoji
				}
			}

			UpdateRoleMenuMessage(ctx, rm)
		}

		return fmt.Sprintf("Sucessfully edited menu, tip: run `rolemenu resetreactions %d` to clear all reactions so that the order is fixed.", rm.MessageID), nil
	}

	return "", nil
}

func updateSetupMessage(ctx context.Context, rm *models.RoleMenu, msgContents string) {
	msgContents = msgContents + "\n\n*This message will be updated with new info throughout the setup.*"

	if rm.SetupMSGID == 0 {
		createSetupMessage(ctx, rm, msgContents, true)
		return
	}

	// if this is a old message, then don't reuse it
	msgAge := bot.SnowflakeToTime(rm.SetupMSGID)
	if msgAge.Before(time.Now().Add(-time.Hour)) {
		createSetupMessage(ctx, rm, msgContents, true)
		return
	}

	_, err := common.BotSession.ChannelMessageEdit(rm.ChannelID, rm.SetupMSGID, msgContents)
	if err != nil {
		createSetupMessage(ctx, rm, msgContents, true)
		return
	}
}

func createSetupMessage(ctx context.Context, rm *models.RoleMenu, msgContents string, updateModel bool) {
	msgContents = "**Rolemenu setup:** " + msgContents

	msg, err := common.BotSession.ChannelMessageSend(rm.ChannelID, msgContents)
	if err != nil {
		logger.WithError(err).WithField("rm_id", rm.MessageID).WithField("guild", rm.GuildID).Error("failed creating setup message for menu")
		return
	}

	if updateModel {
		rm.SetupMSGID = msg.ID
		_, err = rm.UpdateG(ctx, boil.Whitelist("setup_msg_id"))
		if err != nil {
			logger.WithError(err).WithField("rm_id", rm.MessageID).WithField("guild", rm.GuildID).Error("failed upating menu model")
		}
	}
}
