package customcommands

import (
	"context"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var cachedCommandsContextMenuTrigger = common.CacheSet.RegisterSlot("custom_commands_context_menu_trigger", nil, int64(0))

// BotCachedGetCommandsWithContextMenuTrigger returns the (cached) context menu custom
// commands (both user and message type) for a guild.
func BotCachedGetCommandsWithContextMenuTrigger(guildID int64, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := cachedCommandsContextMenuTrigger.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		var cmds []*models.CustomCommand
		var err error
		common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands from db", logrus.Fields{"guild": guildID}, func() {
			cmds, err = models.CustomCommands(
				qm.Where("guild_id = ? AND trigger_type IN (?, ?)", guildID, int(CommandTriggerUserContextMenu), int(CommandTriggerMessageContextMenu)),
				qm.OrderBy("local_id asc"),
				qm.Load("Group"),
			).AllG(ctx)
		})
		return cmds, err
	})
	if err != nil {
		return nil, err
	}
	return v.([]*models.CustomCommand), nil
}

// contextMenuTriggerForCommandType maps a Discord application command type to the
// matching context menu custom command trigger type.
func contextMenuTriggerForCommandType(t discordgo.ApplicationCommandType) (CommandTriggerType, bool) {
	switch t {
	case discordgo.UserApplicationCommand:
		return CommandTriggerUserContextMenu, true
	case discordgo.MessageApplicationCommand:
		return CommandTriggerMessageContextMenu, true
	default:
		return 0, false
	}
}

// handleContextMenuInteraction matches an incoming USER/MESSAGE application command
// interaction to a context menu custom command and executes it.
func handleContextMenuInteraction(evt *eventsystem.EventData, cs *dstate.ChannelState, interaction *templates.CustomCommandInteraction) {
	data := interaction.DataCommand
	if data == nil || interaction.Member == nil {
		return
	}

	wantTrigger, ok := contextMenuTriggerForCommandType(data.CommandType)
	if !ok {
		return
	}

	cmds, err := BotCachedGetCommandsWithContextMenuTrigger(cs.GuildID, evt.Context())
	if err != nil {
		logger.WithField("guild", cs.GuildID).WithError(err).Error("failed fetching context menu ccs")
		return
	}

	var matched *models.CustomCommand
	for _, cmd := range cmds {
		if cmd.TriggerType != int(wantTrigger) {
			continue
		}
		if cmd.Disabled || (cmd.R != nil && cmd.R.Group != nil && cmd.R.Group.Disabled) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cmd.TextTrigger), strings.TrimSpace(data.Name)) {
			matched = cmd
			break
		}
	}

	if matched == nil {
		return
	}

	ms := dstate.MemberStateFromMember(interaction.Member)
	if ms == nil || ms.User.Bot {
		return
	}

	if !CmdRunsInChannel(matched, common.ChannelOrThreadParentID(cs)) || !CmdRunsForUser(matched, ms) {
		respondRestrictedInteraction(interaction)
		return
	}

	deferResponseToCCs(interaction, []*TriggeredCC{{CC: matched}})
	if err := ExecuteCustomCommandFromContextMenu(matched, evt.GS, cs, interaction); err != nil {
		logger.WithField("guild", cs.GuildID).WithField("cc_id", matched.LocalID).WithError(err).Error("Error executing context menu custom command")
	}
}

// ExecuteCustomCommandFromContextMenu builds the template context for a context menu
// custom command and executes it. The template sees:
//
//	.TargetUser    the target's User object — the clicked user for user commands, or the
//	               author of the clicked message for message commands
//	.TargetMember  the matching MemberState for that same target
//	.Author        the User object of the person who invoked the command
//	.Message       the clicked message (message commands only)
//	.CommandType   "user" or "message"
//
// A nil member is passed to NewContext so .User/.Member are not exposed and sendDM (which
// targets the context member) is disabled — a context menu command must not be usable to
// DM an arbitrary target. This mirrors role trigger commands.
func ExecuteCustomCommandFromContextMenu(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, nil)
	tmplCtx.CurrentFrame.Interaction = interaction

	data := interaction.DataCommand

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = data
	tmplCtx.Data["IsContextMenuCommand"] = true
	tmplCtx.Data["CommandName"] = data.Name
	tmplCtx.Data["Cmd"] = data.Name
	if ms != nil {
		tmplCtx.Data["Author"] = &ms.User
	}

	switch data.CommandType {
	case discordgo.UserApplicationCommand:
		tmplCtx.Data["CommandType"] = "user"
		user := resolveSlashUser(data, data.TargetID)
		tmplCtx.Data["TargetUser"] = user
		tmplCtx.Data["TargetMember"], _ = bot.GetMember(gs.ID, user.ID)
	case discordgo.MessageApplicationCommand:
		tmplCtx.Data["CommandType"] = "message"
		if data.Resolved != nil {
			if target, ok := data.Resolved.Messages[data.TargetID]; ok && target != nil {
				target.GuildID = gs.ID
				tmplCtx.Msg = target
				tmplCtx.Data["Message"] = target
				if target.Author != nil {
					tmplCtx.Data["TargetUser"] = target.Author
					tmplCtx.Data["TargetMember"], _ = bot.GetMember(gs.ID, target.Author.ID)
				}
			}
		}
	}

	return ExecuteCustomCommand(cc, tmplCtx)
}

// buildContextMenuCommandRequest converts a stored context menu custom command into the
// discordgo request used to (re)register it with Discord. Context menu commands carry
// no description or options.
func buildContextMenuCommandRequest(cc *models.CustomCommand) *discordgo.CreateApplicationCommandRequest {
	cmdType := discordgo.UserApplicationCommand
	if cc.TriggerType == int(CommandTriggerMessageContextMenu) {
		cmdType = discordgo.MessageApplicationCommand
	}

	defaultPermission := true
	return &discordgo.CreateApplicationCommandRequest{
		Name:              strings.TrimSpace(cc.TextTrigger),
		Type:              cmdType,
		DefaultPermission: &defaultPermission,
	}
}

// buildGuildContextMenuRequests fetches a guild's enabled context menu custom commands
// and builds their registration requests, capping each type at its per-guild limit.
func buildGuildContextMenuRequests(guildID int64) []*discordgo.CreateApplicationCommandRequest {
	cmds, err := models.CustomCommands(
		qm.Where("guild_id = ? AND disabled = false AND trigger_type IN (?, ?)", guildID, int(CommandTriggerUserContextMenu), int(CommandTriggerMessageContextMenu)),
		qm.OrderBy("local_id asc"),
	).AllG(context.Background())
	if err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("failed fetching context menu ccs for resync")
		return nil
	}

	max := MaxContextMenuForContext(guildID)
	var userCount, msgCount int
	set := make([]*discordgo.CreateApplicationCommandRequest, 0, len(cmds))
	for _, cc := range cmds {
		switch cc.TriggerType {
		case int(CommandTriggerUserContextMenu):
			if userCount >= max {
				continue
			}
			userCount++
		case int(CommandTriggerMessageContextMenu):
			if msgCount >= max {
				continue
			}
			msgCount++
		default:
			continue
		}
		set = append(set, buildContextMenuCommandRequest(cc))
	}
	return set
}
