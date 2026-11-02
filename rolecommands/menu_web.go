package rolecommands

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/rolemenus.html
var roleMenusHTML string

var (
	panelLogKeyNewMenu     = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "rolecommands_new_menu", FormatString: "Created a new role menu for group: %s"})
	panelLogKeyUpdatedMenu = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "rolecommands_updated_menu", FormatString: "Updated role menu: %d"})
	panelLogKeyRemovedMenu = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "rolecommands_removed_menu", FormatString: "Removed role menu: %d"})
)

const (
	MaxReactionMenuOptions = 20
	maxRoleMenusPerGuild   = 500
)

var (
	customEmojiRegex = regexp.MustCompile(`^<(a?):(\w{2,32}):(\d{15,22})>$`)
	messageLinkRegex = regexp.MustCompile(`^https://(?:(?:www|ptb|canary)\.)?discord(?:app)?\.com/channels/(\d+)/(\d+)/(\d+)/?$`)
)

type FormRoleMenu struct {
	Kind                       int16
	Group                      int64
	Channel                    int64 `valid:"channel,true"`
	MessageLink                string
	Content                    string
	Placeholder                string
	SendDM                     bool
	RemoveRoleOnReactionRemove bool
	Options                    []FormRoleMenuOption
}

type FormRoleMenuOption struct {
	Include     bool
	RoleCommand int64
	Label       string
	Description string
	Emoji       string
}

type RoleMenuListEntry struct {
	Menu        *models.RoleMenu
	ChannelName string
	Editable    bool
}

type roleMenuCommandData struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type roleMenuOptionData struct {
	Include     bool   `json:"include"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Emoji       string `json:"emoji"`
}

func initRoleMenusWeb(subMux *goji.Mux, groupPageHandler http.Handler) {
	web.AddHTMLTemplate("rolecommands/assets/rolemenus.html", roleMenusHTML)

	editHandler := web.ControllerHandler(handleGetRoleMenu, "cp_rolecommands")
	subMux.Handle(pat.Get("/group/:groupID/menu/:menuID"), editHandler)

	subMux.Handle(pat.Post("/menus/new"), web.ControllerPostHandler(handleNewRoleMenu, groupPageHandler, FormRoleMenu{}))
	subMux.Handle(pat.Post("/menus/:menuID/update"), web.ControllerPostHandler(handleUpdateRoleMenu, editHandler, FormRoleMenu{}))
	subMux.Handle(pat.Post("/menus/:menuID/delete"), web.ControllerPostHandler(handleDeleteRoleMenu, groupPageHandler, nil))
}

func roleMenuDashboardURL(menu *models.RoleMenu) string {
	return fmt.Sprintf("%s/rolecommands/group/%d/menu/%d", web.ManageServerURL(menu.GuildID), menu.RoleGroupID.Int64, menu.MessageID)
}

// setGroupRoleMenusData adds the group's menus and the roles a menu for it can offer to the role commands page.
func setGroupRoleMenusData(ctx context.Context, g *dstate.GuildSet, tmpl web.TemplateData, group *models.RoleGroup, commands []*models.RoleCommand) error {
	menus, err := models.RoleMenus(models.RoleMenuWhere.GuildID.EQ(g.ID), models.RoleMenuWhere.RoleGroupID.EQ(null.Int64From(group.ID)),
		qm.Load(models.RoleMenuRels.RoleMenuOptions), qm.OrderBy(models.RoleMenuColumns.MessageID+" ASC")).AllG(ctx)
	if err != nil {
		return err
	}

	entries := make([]*RoleMenuListEntry, 0, len(menus))
	for _, menu := range menus {
		entry := &RoleMenuListEntry{Menu: menu, Editable: roleMenuEditable(menu)}
		if channel := g.GetChannelOrThread(menu.ChannelID); channel != nil {
			entry.ChannelName = channel.Name
		}
		entries = append(entries, entry)
	}

	commandData := make([]roleMenuCommandData, 0, len(commands))
	for _, cmd := range commands {
		commandData = append(commandData, roleMenuCommandData{ID: cmd.ID, Name: cmd.Name})
	}

	tmpl["RoleMenus"] = entries
	tmpl["RoleMenuCommands"] = commandData
	tmpl["MaxReactionMenuOptions"] = MaxReactionMenuOptions
	tmpl["MaxDropdownOptions"] = MaxDropdownOptions
	return nil
}

func handleGetRoleMenu(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	menu, err := findGuildRoleMenu(ctx, g.ID, r)
	if err != nil {
		return tmpl, err
	}

	tmpl, err = HandleGetGroup(menu.RoleGroupID.Int64, w, r)
	if err != nil || !roleMenuEditable(menu) {
		return tmpl, err
	}

	if _, ok := tmpl["RoleMenuDraft"]; !ok {
		draft := &FormRoleMenu{
			Kind:                       menu.Kind,
			Group:                      menu.RoleGroupID.Int64,
			Placeholder:                menu.Placeholder,
			SendDM:                     !menu.DisableSendDM,
			RemoveRoleOnReactionRemove: menu.RemoveRoleOnReactionRemove,
		}
		if menu.OwnMessage {
			draft.Content = menu.SavedContent.String
		}
		for _, option := range sortedMenuOptions(menu) {
			draft.Options = append(draft.Options, FormRoleMenuOption{
				Include:     true,
				RoleCommand: option.RoleCommandID.Int64,
				Label:       option.Label,
				Description: option.Description,
				Emoji:       optionEmojiText(g, option),
			})
		}
		setRoleMenuDraft(tmpl, draft)
	}

	entry := &RoleMenuListEntry{Menu: menu, Editable: true}
	if channel := g.GetChannelOrThread(menu.ChannelID); channel != nil {
		entry.ChannelName = channel.Name
	}
	tmpl["EditMenu"] = entry
	return tmpl, nil
}

// setRoleMenuDraft puts form values on the page, so a form that failed to save keeps what was entered.
func setRoleMenuDraft(tmpl web.TemplateData, form *FormRoleMenu) {
	options := make(map[string]roleMenuOptionData, len(form.Options))
	for _, option := range form.Options {
		options[strconv.FormatInt(option.RoleCommand, 10)] = roleMenuOptionData{
			Include:     option.Include,
			Label:       option.Label,
			Description: option.Description,
			Emoji:       option.Emoji,
		}
	}

	tmpl["RoleMenuDraft"] = form
	tmpl["RoleMenuDraftOptions"] = options
}

func findGuildRoleMenu(ctx context.Context, guildID int64, r *http.Request) (*models.RoleMenu, error) {
	menuID, _ := strconv.ParseInt(pat.Param(r, "menuID"), 10, 64)
	menu, err := models.RoleMenus(models.RoleMenuWhere.GuildID.EQ(guildID), models.RoleMenuWhere.MessageID.EQ(menuID),
		qm.Load(models.RoleMenuRels.RoleMenuOptions+"."+models.RoleMenuOptionRels.RoleCommand), qm.Load(models.RoleMenuRels.RoleGroup+"."+models.RoleGroupRels.RoleCommands)).OneG(ctx)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, web.NewPublicError("That role menu doesn't exist anymore")
		}
		return nil, err
	}

	return menu, nil
}

// roleMenuEditable is false for menus the dashboard can't represent: ones without a group, or still being set up by command.
func roleMenuEditable(menu *models.RoleMenu) bool {
	return menu.RoleGroupID.Valid && menu.State == RoleMenuStateDone
}

func optionEmojiText(gs *dstate.GuildSet, option *models.RoleMenuOption) string {
	if option.EmojiID == 0 {
		return option.UnicodeEmoji
	}

	name := "emoji"
	if emoji := gs.GetEmoji(option.EmojiID); emoji != nil {
		name = emoji.Name
	}

	return (&discordgo.Emoji{ID: option.EmojiID, Name: name, Animated: option.EmojiAnimated}).MessageFormat()
}

// parseOptionEmoji accepts a unicode emoji or a custom emoji in its <:name:id> message format.
func parseOptionEmoji(input string) (emojiID int64, unicode string, animated bool, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, "", false, nil
	}

	if m := customEmojiRegex.FindStringSubmatch(input); m != nil {
		emojiID, _ = strconv.ParseInt(m[3], 10, 64)
		return emojiID, "", m[1] == "a", nil
	}

	if strings.ContainsAny(input, " <>:") || utf8.RuneCountInString(input) > 10 {
		return 0, "", false, errors.New("isn't an emoji")
	}

	for _, r := range input {
		if r < 0x80 {
			return 0, "", false, errors.New("isn't an emoji")
		}
	}

	return 0, input, false, nil
}

func parseMessageLink(link string) (guildID, channelID, messageID int64, err error) {
	m := messageLinkRegex.FindStringSubmatch(strings.TrimSpace(link))
	if m == nil {
		return 0, 0, 0, errors.New("not a valid message link")
	}

	guildID, _ = strconv.ParseInt(m[1], 10, 64)
	channelID, _ = strconv.ParseInt(m[2], 10, 64)
	messageID, _ = strconv.ParseInt(m[3], 10, 64)
	return guildID, channelID, messageID, nil
}

// buildRoleMenuOptions validates the form options against the group and turns them into option models.
func buildRoleMenuOptions(gs *dstate.GuildSet, group *models.RoleGroup, kind int16, formOptions []FormRoleMenuOption) ([]*models.RoleMenuOption, error) {
	commands := make(map[int64]*models.RoleCommand)
	for _, cmd := range group.R.RoleCommands {
		commands[cmd.ID] = cmd
	}

	var options []*models.RoleMenuOption
	seenCommands := make(map[int64]bool)
	seenEmojis := make(map[string]bool)
	for _, formOption := range formOptions {
		if !formOption.Include {
			continue
		}

		cmd := commands[formOption.RoleCommand]
		if cmd == nil || seenCommands[cmd.ID] {
			continue
		}
		seenCommands[cmd.ID] = true

		label := strings.TrimSpace(formOption.Label)
		description := strings.TrimSpace(formOption.Description)
		if utf8.RuneCountInString(label) > MaxDropdownLabelLength {
			return nil, web.NewPublicError(fmt.Sprintf("The label for %q can be at most %d characters", cmd.Name, MaxDropdownLabelLength))
		}
		if utf8.RuneCountInString(description) > MaxDropdownDescription {
			return nil, web.NewPublicError(fmt.Sprintf("The description for %q can be at most %d characters", cmd.Name, MaxDropdownDescription))
		}

		emojiID, unicode, animated, err := parseOptionEmoji(formOption.Emoji)
		if err != nil {
			return nil, web.NewPublicError(fmt.Sprintf("The emoji for %q %s, pick one with the emoji picker", cmd.Name, err))
		}
		if emojiID != 0 && gs.GetEmoji(emojiID) == nil {
			return nil, web.NewPublicError(fmt.Sprintf("The emoji for %q isn't from this server, only this server's custom emojis can be used", cmd.Name))
		}

		emojiKey := unicode
		if emojiID != 0 {
			emojiKey = strconv.FormatInt(emojiID, 10)
		}

		if kind == RoleMenuKindReactions {
			if emojiKey == "" {
				return nil, web.NewPublicError(fmt.Sprintf("%q needs an emoji, members react with it to get the role", cmd.Name))
			}
			if seenEmojis[emojiKey] {
				return nil, web.NewPublicError(fmt.Sprintf("%q uses an emoji another option already has, each option needs its own", cmd.Name))
			}
			label, description = "", ""
		}
		if emojiKey != "" {
			seenEmojis[emojiKey] = true
		}

		option := &models.RoleMenuOption{
			RoleCommandID: null.Int64From(cmd.ID),
			EmojiID:       emojiID,
			UnicodeEmoji:  unicode,
			EmojiAnimated: animated,
			Label:         label,
			Description:   description,
		}
		option.R = option.R.NewStruct()
		option.R.RoleCommand = cmd
		options = append(options, option)
	}

	limit := MaxDropdownOptions
	if kind == RoleMenuKindReactions {
		limit = MaxReactionMenuOptions
	}

	if len(options) < 1 {
		return nil, web.NewPublicError("Turn on at least one role for the menu")
	}
	if len(options) > limit {
		return nil, web.NewPublicError(fmt.Sprintf("This kind of menu can have at most %d roles, turn some of them off", limit))
	}

	return options, nil
}

func validateRoleMenuText(form *FormRoleMenu) error {
	form.Content = strings.TrimSpace(form.Content)
	form.Placeholder = strings.TrimSpace(form.Placeholder)
	if utf8.RuneCountInString(form.Content) > 2000 {
		return web.NewPublicError("The message text can be at most 2000 characters")
	}
	if utf8.RuneCountInString(form.Placeholder) > MaxDropdownPlaceholder {
		return web.NewPublicError(fmt.Sprintf("The placeholder can be at most %d characters", MaxDropdownPlaceholder))
	}
	return nil
}

func defaultRoleMenuContent(menu *models.RoleMenu, gs *dstate.GuildSet) string {
	if isDropdownMenu(menu) {
		return "**" + menu.R.RoleGroup.Name + "**\nPick a role from the menu below, pick it again to remove it."
	}

	var b strings.Builder
	b.WriteString("**Role Menu: " + menu.R.RoleGroup.Name + "**\nReact to give yourself a role.\n\n")
	for _, option := range sortedMenuOptions(menu) {
		fmt.Fprintf(&b, "%s : `%s`\n\n", optionEmojiText(gs, option), option.R.RoleCommand.Name)
	}
	return b.String()
}

func sortedMenuOptions(menu *models.RoleMenu) []*models.RoleMenuOption {
	options := append([]*models.RoleMenuOption(nil), menu.R.RoleMenuOptions...)
	sort.Slice(options, OptionsLessFunc(false, options))
	return options
}

func reactionAPIName(option *models.RoleMenuOption) string {
	if option.EmojiID != 0 {
		return "emoji:" + strconv.FormatInt(option.EmojiID, 10)
	}
	return option.UnicodeEmoji
}

func humanizeRoleMenuDiscordError(err error) error {
	code, msg := common.DiscordError(err)
	switch code {
	case 0:
		return err
	case discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions:
		return web.NewPublicError("The bot is missing permissions in that channel, it needs to see the channel, send messages, add reactions and read message history")
	case discordgo.ErrCodeUnknownMessage, discordgo.ErrCodeUnknownChannel:
		return web.NewPublicError("Couldn't find that message, it may have been deleted")
	case discordgo.ErrCodeUnknownEmoji:
		return web.NewPublicError("One of the emojis couldn't be used, pick it again with the emoji picker")
	case discordgo.ErrCodeTooManyReactions:
		return web.NewPublicError("That message has too many different reactions on it, remove some first")
	}
	return web.NewPublicError("Discord rejected the menu: " + msg)
}

func handleNewRoleMenu(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	form := ctx.Value(common.ContextKeyParsedForm).(*FormRoleMenu)
	defer func() {
		if err != nil {
			setRoleMenuDraft(tmpl, form)
		}
	}()

	if form.Kind != RoleMenuKindReactions && form.Kind != RoleMenuKindDropdown {
		return tmpl, web.NewPublicError("Unknown menu type")
	}
	if err := validateRoleMenuText(form); err != nil {
		return tmpl, err
	}

	if count, err := models.RoleMenus(models.RoleMenuWhere.GuildID.EQ(g.ID)).CountG(ctx); err != nil {
		return tmpl, err
	} else if count >= maxRoleMenusPerGuild {
		return tmpl, web.NewPublicError(fmt.Sprintf("A server can have at most %d role menus", maxRoleMenusPerGuild))
	}

	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return tmpl, err
	}
	defer tx.Rollback()

	group, err := models.RoleGroups(models.RoleGroupWhere.GuildID.EQ(g.ID), models.RoleGroupWhere.ID.EQ(form.Group), qm.Load(models.RoleGroupRels.RoleCommands)).One(ctx, tx)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return tmpl, web.NewPublicError("That role group doesn't exist anymore")
		}
		return tmpl, err
	}

	options, err := buildRoleMenuOptions(g, group, form.Kind, form.Options)
	if err != nil {
		return tmpl, err
	}

	menu := &models.RoleMenu{
		GuildID:                    g.ID,
		OwnerID:                    web.ContextUser(ctx).ID,
		State:                      RoleMenuStateDone,
		RoleGroupID:                null.Int64From(group.ID),
		Kind:                       form.Kind,
		DisableSendDM:              !form.SendDM,
		RemoveRoleOnReactionRemove: form.RemoveRoleOnReactionRemove,
	}
	if form.Kind == RoleMenuKindDropdown {
		menu.Placeholder = form.Placeholder
		menu.DisableSendDM = true
		menu.RemoveRoleOnReactionRemove = false
	}
	menu.R = menu.R.NewStruct()
	menu.R.RoleGroup = group
	menu.R.RoleMenuOptions = options

	if strings.TrimSpace(form.MessageLink) != "" {
		err = attachRoleMenuToMessage(ctx, g, menu, form.MessageLink)
	} else {
		err = sendRoleMenuMessage(g, menu, form)
	}
	if err != nil {
		return tmpl, err
	}

	err = insertRoleMenu(ctx, tx, menu, options)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if menu.OwnMessage {
			common.BotSession.ChannelMessageDelete(menu.ChannelID, menu.MessageID)
		} else if isDropdownMenu(menu) {
			removeRoleMenuDropdown(menu)
		}
		if common.ErrPQIsUniqueViolation(err) {
			return tmpl, web.NewPublicError("That message already has a role menu, edit that one instead")
		}
		return tmpl, err
	}

	if menu.Kind == RoleMenuKindReactions {
		if err := addRoleMenuReactions(menu, options); err != nil {
			web.CtxLogger(ctx).WithError(err).Warn("failed adding role menu reactions")
			tmpl.AddAlerts(web.WarningAlert("The menu was created, but some reactions couldn't be added: ", humanizeRoleMenuDiscordError(err).Error()))
		}
	}

	sendEvictMenuCachePubSub(g.ID)
	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyNewMenu, &cplogs.Param{Type: cplogs.ParamTypeString, Value: group.Name}))
	return tmpl, nil
}

func insertRoleMenu(ctx context.Context, exec boil.ContextExecutor, menu *models.RoleMenu, options []*models.RoleMenuOption) error {
	if err := menu.Insert(ctx, exec, boil.Infer()); err != nil {
		return err
	}

	for _, option := range options {
		option.RoleMenuID = menu.MessageID
		if err := option.Insert(ctx, exec, boil.Infer()); err != nil {
			return err
		}
	}

	return nil
}

func sendRoleMenuMessage(gs *dstate.GuildSet, menu *models.RoleMenu, form *FormRoleMenu) error {
	channel := gs.GetChannel(form.Channel)
	if channel == nil {
		return web.NewPublicError("Pick a channel to send the menu in")
	}

	menu.ChannelID = channel.ID
	menu.OwnMessage = true

	content := form.Content
	if content != "" {
		menu.SavedContent = null.StringFrom(content)
	} else {
		content = defaultRoleMenuContent(menu, gs)
	}

	send := &discordgo.MessageSend{
		Content:         content,
		AllowedMentions: discordgo.AllowedMentions{},
	}
	if isDropdownMenu(menu) {
		send.Components = []discordgo.TopLevelComponent{roleMenuDropdownRow(menu)}
	}

	msg, err := common.BotSession.ChannelMessageSendComplex(channel.ID, send)
	if err != nil {
		return humanizeRoleMenuDiscordError(err)
	}

	menu.MessageID = msg.ID
	return nil
}

func attachRoleMenuToMessage(ctx context.Context, gs *dstate.GuildSet, menu *models.RoleMenu, link string) error {
	guildID, channelID, messageID, err := parseMessageLink(link)
	if err != nil {
		return web.NewPublicError("That message link isn't valid, copy it with Copy Message Link in Discord")
	}
	if guildID != gs.ID || gs.GetChannelOrThread(channelID) == nil {
		return web.NewPublicError("That message isn't in this server")
	}

	if exists, err := models.RoleMenus(models.RoleMenuWhere.MessageID.EQ(messageID)).ExistsG(ctx); err != nil {
		return err
	} else if exists {
		return web.NewPublicError("That message already has a role menu, edit that one instead")
	}

	msg, err := common.BotSession.ChannelMessage(channelID, messageID)
	if err != nil {
		return humanizeRoleMenuDiscordError(err)
	}

	menu.ChannelID = channelID
	menu.MessageID = msg.ID
	menu.OwnMessage = false

	if !isDropdownMenu(menu) {
		return nil
	}

	if msg.Author == nil || msg.Author.ID != common.BotUser.ID {
		return web.NewPublicError("Select menus can only be added to messages the bot sent, send one with the Message Creator first or let the bot send a new message")
	}

	return humanizeRoleMenuDiscordErrorOrNil(updateRoleMenuDropdownMessage(menu, nil))
}

func humanizeRoleMenuDiscordErrorOrNil(err error) error {
	if err == nil {
		return nil
	}
	return humanizeRoleMenuDiscordError(err)
}

func addRoleMenuReactions(menu *models.RoleMenu, options []*models.RoleMenuOption) error {
	for _, option := range options {
		if err := common.BotSession.MessageReactionAdd(menu.ChannelID, menu.MessageID, reactionAPIName(option)); err != nil {
			return err
		}
	}
	return nil
}

func handleUpdateRoleMenu(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	form := ctx.Value(common.ContextKeyParsedForm).(*FormRoleMenu)
	defer func() {
		if err != nil {
			setRoleMenuDraft(tmpl, form)
		}
	}()

	menu, err := findGuildRoleMenu(ctx, g.ID, r)
	if err != nil {
		return tmpl, err
	}
	if !roleMenuEditable(menu) || menu.R.RoleGroup == nil {
		return tmpl, web.NewPublicError("This menu can't be edited on the dashboard")
	}
	if err := validateRoleMenuText(form); err != nil {
		return tmpl, err
	}

	options, err := buildRoleMenuOptions(g, menu.R.RoleGroup, menu.Kind, form.Options)
	if err != nil {
		return tmpl, err
	}

	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return tmpl, err
	}
	defer tx.Rollback()

	oldOptions := menu.R.RoleMenuOptions
	if isDropdownMenu(menu) {
		menu.Placeholder = form.Placeholder
	} else {
		menu.DisableSendDM = !form.SendDM
		menu.RemoveRoleOnReactionRemove = form.RemoveRoleOnReactionRemove
	}
	if menu.OwnMessage {
		menu.SavedContent = null.NewString(form.Content, form.Content != "")
	}

	if _, err := menu.Update(ctx, tx, boil.Infer()); err != nil {
		return tmpl, err
	}
	if _, err := models.RoleMenuOptions(models.RoleMenuOptionWhere.RoleMenuID.EQ(menu.MessageID)).DeleteAll(ctx, tx); err != nil {
		return tmpl, err
	}
	for _, option := range options {
		option.RoleMenuID = menu.MessageID
		if err := option.Insert(ctx, tx, boil.Infer()); err != nil {
			return tmpl, err
		}
	}
	if err := tx.Commit(); err != nil {
		return tmpl, err
	}

	menu.R.RoleMenuOptions = options
	sendEvictMenuCachePubSub(g.ID)
	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyUpdatedMenu, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: menu.MessageID}))

	if err := redrawRoleMenu(g, menu, oldOptions); err != nil {
		web.CtxLogger(ctx).WithError(err).Warn("failed redrawing role menu")
		tmpl.AddAlerts(web.WarningAlert("The menu was saved, but its message couldn't be updated: ", humanizeRoleMenuDiscordError(err).Error()))
	}

	return tmpl, nil
}

func redrawRoleMenu(gs *dstate.GuildSet, menu *models.RoleMenu, oldOptions []*models.RoleMenuOption) error {
	var content *string
	if menu.OwnMessage {
		text := menu.SavedContent.String
		if text == "" && menu.SavedEmbed.String == "" {
			text = defaultRoleMenuContent(menu, gs)
		}
		content = &text
	}

	if isDropdownMenu(menu) {
		return updateRoleMenuDropdownMessage(menu, content)
	}

	if content != nil {
		_, err := common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:              menu.MessageID,
			Channel:         menu.ChannelID,
			Content:         content,
			AllowedMentions: discordgo.AllowedMentions{},
		})
		if err != nil {
			return err
		}
	}

	kept := make(map[string]bool)
	for _, option := range menu.R.RoleMenuOptions {
		kept[reactionAPIName(option)] = true
	}
	for _, option := range oldOptions {
		if name := reactionAPIName(option); !kept[name] {
			if err := common.BotSession.MessageReactionRemoveEmoji(menu.ChannelID, menu.MessageID, name); err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeUnknownEmoji) {
				return err
			}
		}
	}

	return addRoleMenuReactions(menu, sortedMenuOptions(menu))
}

func handleDeleteRoleMenu(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	menu, err := findGuildRoleMenu(ctx, g.ID, r)
	if err != nil {
		return tmpl, err
	}

	if _, err := menu.DeleteG(ctx); err != nil {
		return tmpl, err
	}

	sendEvictMenuCachePubSub(g.ID)
	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyRemovedMenu, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: menu.MessageID}))

	if r.FormValue("DeleteMessage") != "" && menu.OwnMessage {
		err = common.BotSession.ChannelMessageDelete(menu.ChannelID, menu.MessageID)
	} else if isDropdownMenu(menu) {
		err = removeRoleMenuDropdown(menu)
	}
	if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeUnknownMessage, discordgo.ErrCodeUnknownChannel) {
		web.CtxLogger(ctx).WithError(err).Warn("failed cleaning up deleted role menu message")
		tmpl.AddAlerts(web.WarningAlert("The menu was deleted, but its message couldn't be cleaned up: ", humanizeRoleMenuDiscordError(err).Error()))
	}

	return tmpl, nil
}

func removeRoleMenuDropdown(menu *models.RoleMenu) error {
	msg, err := common.BotSession.ChannelMessage(menu.ChannelID, menu.MessageID)
	if err != nil {
		return err
	}

	edit := &discordgo.MessageEdit{
		ID:              menu.MessageID,
		Channel:         menu.ChannelID,
		Components:      withoutRoleMenuDropdown(msg.Components),
		AllowedMentions: discordgo.AllowedMentions{},
		Flags:           msg.Flags & (discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsIsComponentsV2),
	}
	if msg.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 {
		edit.Content = &msg.Content
	}

	_, err = common.BotSession.ChannelMessageEditComplex(edit)
	return err
}
