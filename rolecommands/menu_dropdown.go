package rolecommands

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands/models"
)

const (
	RoleMenuKindReactions int16 = 0
	RoleMenuKindDropdown  int16 = 1

	roleMenuDropdownCustomID = "rolemenu-dropdown"

	MaxDropdownOptions         = 25
	MaxDropdownLabelLength     = 100
	MaxDropdownDescription     = 100
	MaxDropdownPlaceholder     = 150
	defaultDropdownPlaceholder = "Pick a role"
)

func isDropdownMenu(menu *models.RoleMenu) bool {
	return menu.Kind == RoleMenuKindDropdown
}

func dropdownMenuDashboardHint(menu *models.RoleMenu) string {
	return fmt.Sprintf("This is a select menu role menu, change it on the control panel: <%s>", roleMenuDashboardURL(menu))
}

// roleMenuDropdownRow assumes the options are loaded with their role commands and the group with its mode.
func roleMenuDropdownRow(menu *models.RoleMenu) discordgo.ActionsRow {
	options := slices.Clone(menu.R.RoleMenuOptions)
	options = slices.DeleteFunc(options, func(o *models.RoleMenuOption) bool { return o.R == nil || o.R.RoleCommand == nil })
	sort.Slice(options, OptionsLessFunc(false, options))

	selectOptions := make([]discordgo.SelectMenuOption, 0, len(options))
	for _, option := range options {
		label := option.Label
		if label == "" {
			label = option.R.RoleCommand.Name
		}

		selectOption := discordgo.SelectMenuOption{
			Label:       common.CutStringShort(label, MaxDropdownLabelLength),
			Value:       strconv.FormatInt(option.R.RoleCommand.ID, 10),
			Description: option.Description,
		}
		if option.EmojiID != 0 || option.UnicodeEmoji != "" {
			selectOption.Emoji = &discordgo.ComponentEmoji{ID: option.EmojiID, Name: option.UnicodeEmoji, Animated: option.EmojiAnimated}
		}
		selectOptions = append(selectOptions, selectOption)
	}

	maxValues := len(selectOptions)
	if menu.R.RoleGroup != nil && menu.R.RoleGroup.Mode == GroupModeSingle {
		maxValues = 1
	}

	placeholder := menu.Placeholder
	if placeholder == "" {
		placeholder = defaultDropdownPlaceholder
	}

	minValues := 1
	return discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
		discordgo.SelectMenu{
			MenuType:    discordgo.StringSelectMenu,
			CustomID:    roleMenuDropdownCustomID,
			Placeholder: placeholder,
			MinValues:   &minValues,
			MaxValues:   maxValues,
			Options:     selectOptions,
		},
	}}
}

// withRoleMenuDropdown replaces the menu's dropdown in a message's components,
// adding it at the end if the message does not have it yet.
func withRoleMenuDropdown(components []discordgo.TopLevelComponent, menu *models.RoleMenu) []discordgo.TopLevelComponent {
	result := withoutRoleMenuDropdown(components)
	return append(result, roleMenuDropdownRow(menu))
}

func withoutRoleMenuDropdown(components []discordgo.TopLevelComponent) []discordgo.TopLevelComponent {
	return slices.DeleteFunc(slices.Clone(components), isRoleMenuDropdownRow)
}

func isRoleMenuDropdownRow(component discordgo.TopLevelComponent) bool {
	var row discordgo.ActionsRow
	switch v := component.(type) {
	case discordgo.ActionsRow:
		row = v
	case *discordgo.ActionsRow:
		row = *v
	default:
		return false
	}

	for _, inner := range row.Components {
		switch menu := inner.(type) {
		case discordgo.SelectMenu:
			if menu.CustomID == roleMenuDropdownCustomID {
				return true
			}
		case *discordgo.SelectMenu:
			if menu.CustomID == roleMenuDropdownCustomID {
				return true
			}
		}
	}

	return false
}

// updateRoleMenuDropdownMessage redraws the dropdown on the menu's message, keeping anything else on it.
func updateRoleMenuDropdownMessage(menu *models.RoleMenu, content *string) error {
	msg, err := common.BotSession.ChannelMessage(menu.ChannelID, menu.MessageID)
	if err != nil {
		return err
	}

	edit := &discordgo.MessageEdit{
		ID:              menu.MessageID,
		Channel:         menu.ChannelID,
		Content:         content,
		Components:      withRoleMenuDropdown(msg.Components, menu),
		AllowedMentions: discordgo.AllowedMentions{},
		// Flags are always sent on an edit, these are the only ones it may carry.
		Flags: msg.Flags & (discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsIsComponentsV2),
	}
	if content == nil && msg.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 {
		edit.Content = &msg.Content
	}

	_, err = common.BotSession.ChannelMessageEditComplex(edit)
	return err
}

func handleRoleMenuDropdownInteraction(evt *eventsystem.EventData) (retry bool, err error) {
	ic := evt.InteractionCreate()
	if ic.GuildID == 0 || ic.Type != discordgo.InteractionMessageComponent || ic.Member == nil || ic.Member.User == nil || ic.Message == nil {
		return false, nil
	}

	data := ic.MessageComponentData()
	if data.CustomID != roleMenuDropdownCustomID {
		return false, nil
	}

	gs := bot.State.GetGuild(ic.GuildID)
	if gs == nil {
		return false, nil
	}

	ctx := evt.Context()
	// The custom id alone proves nothing, as the message creator lets anyone use it.
	menu, err := GetRolemenuCached(ctx, gs, ic.Message.ID)
	if err != nil {
		return true, err
	}
	if menu == nil || !isDropdownMenu(menu) || menu.MessageID != ic.Message.ID {
		return false, nil
	}

	// Answering by redrawing the message clears the member's pick, so the same option can be picked again later.
	// Content and embeds are always sent with this response, so they are passed back unchanged.
	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:         ic.Message.Content,
			Embeds:          ic.Message.Embeds,
			Components:      ic.Message.Components,
			AllowedMentions: &discordgo.AllowedMentions{},
		},
	})
	if err != nil {
		return false, err
	}

	ms := dstate.MemberStateFromMember(ic.Member)
	ms.GuildID = ic.GuildID
	if ms.User.Bot {
		return false, nil
	}

	results := toggleDropdownPicks(ctx, gs, menu, ms, data.Values)
	go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "user_interacted_menu")

	if len(results) == 0 {
		return false, nil
	}

	_, err = evt.Session.CreateFollowupMessage(common.BotApplication.ID, ic.Token, &discordgo.WebhookParams{
		Content:         strings.Join(results, "\n"),
		Flags:           discordgo.MessageFlagsEphemeral,
		AllowedMentions: &discordgo.AllowedMentions{},
	})
	return false, err
}

// toggleDropdownPicks toggles each picked role the same way reacting does,
// keeping the member's roles current between picks so the group rules see them.
func toggleDropdownPicks(ctx context.Context, gs *dstate.GuildSet, menu *models.RoleMenu, ms *dstate.MemberState, picked []string) []string {
	var results []string
	for _, value := range picked {
		roleCommandID, _ := strconv.ParseInt(value, 10, 64)
		option := dropdownOption(menu, roleCommandID)
		if option == nil {
			results = append(results, "That role isn't on this menu anymore.")
			continue
		}

		settings := CommonRoleFromRoleCommand(menu.R.RoleGroup, option.R.RoleCommand)
		given, err := settings.CheckToggleRole(ctx, ms)
		if err != nil {
			reason, err := HumanizeAssignError(gs, err)
			if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingPermissions) {
				logger.WithError(err).WithField("guild", gs.ID).Error("failed toggling role from dropdown menu")
			}
			results = append(results, fmt.Sprintf("<@&%d>: %s", settings.RoleId, reason))
			continue
		}

		ms.Member.Roles = rolesAfterToggle(ctx, settings, ms.Member.Roles, given)
		if given {
			results = append(results, fmt.Sprintf("Gave you <@&%d>", settings.RoleId))
		} else {
			results = append(results, fmt.Sprintf("Took away <@&%d>", settings.RoleId))
		}
	}

	return results
}

func dropdownOption(menu *models.RoleMenu, roleCommandID int64) *models.RoleMenuOption {
	for _, option := range menu.R.RoleMenuOptions {
		if option.R != nil && option.R.RoleCommand != nil && option.R.RoleCommand.ID == roleCommandID {
			return option
		}
	}

	return nil
}

// rolesAfterToggle mirrors what GroupToggleRole did, including dropping the
// other roles of a single choice group.
func rolesAfterToggle(ctx context.Context, settings *CommonRoleSettings, roles []int64, given bool) []int64 {
	roles = slices.Clone(roles)
	if !given {
		return slices.DeleteFunc(roles, func(id int64) bool { return id == settings.RoleId })
	}

	if settings.ParentGroupMode == GroupModeSingle {
		for _, other := range settings.AllGroupRoles(ctx) {
			roles = slices.DeleteFunc(roles, func(id int64) bool { return id == other.RoleId })
		}
	}

	return append(roles, settings.RoleId)
}
