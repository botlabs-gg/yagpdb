package customcommands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// SlashCommandResyncEvent is the pubsub event published (with the guild as the
// target) whenever a guild's slash command custom commands change and the set
// registered with Discord needs to be rebuilt. It is handled on the bot node
// that owns the guild's shard, see BotInit.
const SlashCommandResyncEvent = "custom_commands_resync_slash"

var cachedCommandsSlashTrigger = common.CacheSet.RegisterSlot("custom_commands_slash_trigger", nil, int64(0))

// BotCachedGetCommandsWithSlashTrigger returns the (cached) slash command custom
// commands for a guild.
func BotCachedGetCommandsWithSlashTrigger(guildID int64, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := cachedCommandsSlashTrigger.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		var cmds []*models.CustomCommand
		var err error
		common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands from db", logrus.Fields{"guild": guildID}, func() {
			cmds, err = models.CustomCommands(qm.Where("guild_id = ? AND trigger_type = ?", guildID, int(CommandTriggerSlash)), qm.OrderBy("local_id asc"), qm.Load("Group")).AllG(ctx)
		})
		return cmds, err
	})
	if err != nil {
		return nil, err
	}
	return v.([]*models.CustomCommand), nil
}

// handleSlashCommandInteraction matches an incoming application command interaction
// to a slash command custom command and executes it. It is dispatched from
// handleInteractionCreate for InteractionApplicationCommand interactions.
func handleSlashCommandInteraction(evt *eventsystem.EventData, cs *dstate.ChannelState, interaction *templates.CustomCommandInteraction) {
	data := interaction.DataCommand
	if data == nil || interaction.Member == nil {
		return
	}

	// USER (2) and MESSAGE (3) application commands are context menu commands and are
	// handled separately; everything else is a slash (CHAT_INPUT) command.
	if data.CommandType == discordgo.UserApplicationCommand || data.CommandType == discordgo.MessageApplicationCommand {
		handleContextMenuInteraction(evt, cs, interaction)
		return
	}

	cmds, err := BotCachedGetCommandsWithSlashTrigger(cs.GuildID, evt.Context())
	if err != nil {
		logger.WithField("guild", cs.GuildID).WithError(err).Error("failed fetching slash command ccs")
		return
	}

	var matched *models.CustomCommand
	for _, cmd := range cmds {
		if cmd.Disabled || (cmd.R != nil && cmd.R.Group != nil && cmd.R.Group.Disabled) {
			continue
		}
		if strings.EqualFold(cmd.TextTrigger, data.Name) {
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

	// Channel/role restrictions: tell the user why instead of failing silently.
	if !CmdRunsInChannel(matched, common.ChannelOrThreadParentID(cs)) || !CmdRunsForUser(matched, ms) {
		respondRestrictedInteraction(interaction)
		return
	}

	deferResponseToCCs(interaction, []*TriggeredCC{{CC: matched}})
	if err := ExecuteCustomCommandFromSlash(matched, evt.GS, cs, interaction); err != nil {
		logger.WithField("guild", cs.GuildID).WithField("cc_id", matched.LocalID).WithError(err).Error("Error executing slash command custom command")
	}
}

// ExecuteCustomCommandFromSlash builds the template context for a slash command
// custom command and executes it. Option values are exposed in .Options (keyed by
// option name); .Args is the ordered slice (command name at index 0) and .CmdArgs
// is the ordered option values.
func ExecuteCustomCommandFromSlash(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, ms)
	tmplCtx.CurrentFrame.Interaction = interaction

	data := interaction.DataCommand

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = data
	tmplCtx.Data["IsSlashCommand"] = true
	tmplCtx.Data["CommandName"] = data.Name
	tmplCtx.Data["Cmd"] = data.Name

	stored := parseSlashCommandData(cc)

	// For a command with subcommands the chosen subcommand is the first interaction
	// option (type SUB_COMMAND); descend into it for the leaf options and expose its
	// name as .SubCommand. Otherwise use the top-level options directly.
	defs := stored.Options
	leafOptions := data.Options
	subName := ""
	if len(stored.Subcommands) > 0 && len(data.Options) > 0 && data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		subName = data.Options[0].Name
		leafOptions = data.Options[0].Options
		for _, s := range stored.Subcommands {
			if strings.EqualFold(s.Name, subName) {
				defs = s.Options
				break
			}
		}
	}
	tmplCtx.Data["SubCommand"] = subName

	provided := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(leafOptions))
	for _, o := range leafOptions {
		provided[strings.ToLower(o.Name)] = o
	}

	options := templates.SDict{}
	args := make([]interface{}, 0, len(defs)+1)
	args = append(args, data.Name)
	for _, def := range defs {
		o, ok := provided[strings.ToLower(def.Name)]
		if !ok {
			// optional option not provided by the user
			continue
		}
		val := resolveSlashOptionValue(o, data)
		options[def.Name] = val
		args = append(args, val)
	}
	tmplCtx.Data["Options"] = options
	tmplCtx.Data["Args"] = args
	tmplCtx.Data["CmdArgs"] = args[1:]

	// application command interactions carry no source message, build a minimal one
	// so member/author template helpers keep working.
	msg := &discordgo.Message{
		GuildID:   gs.ID,
		ChannelID: cs.ID,
		Member:    ms.DgoMember(),
	}
	msg.Author = msg.Member.User
	tmplCtx.Msg = msg
	tmplCtx.Data["Message"] = msg

	return ExecuteCustomCommand(cc, tmplCtx)
}

// resolveSlashOptionValue converts a provided interaction option into the value
// exposed to templates, resolving snowflake-typed options against the resolved
// data Discord sent with the interaction.
func resolveSlashOptionValue(o *discordgo.ApplicationCommandInteractionDataOption, data *discordgo.ApplicationCommandInteractionData) interface{} {
	switch o.Type {
	case discordgo.ApplicationCommandOptionString:
		if v, ok := o.Value.(string); ok {
			return v
		}
	case discordgo.ApplicationCommandOptionInteger:
		if v, ok := o.Value.(int64); ok {
			return v
		}
	case discordgo.ApplicationCommandOptionNumber:
		if v, ok := o.Value.(float64); ok {
			return v
		}
	case discordgo.ApplicationCommandOptionBoolean:
		if v, ok := o.Value.(bool); ok {
			return v
		}
	case discordgo.ApplicationCommandOptionUser:
		id, _ := o.Value.(int64)
		return resolveSlashUser(data, id)
	case discordgo.ApplicationCommandOptionChannel:
		id, _ := o.Value.(int64)
		if data.Resolved != nil {
			if c, ok := data.Resolved.Channels[id]; ok {
				return c
			}
		}
		return &discordgo.Channel{ID: id}
	case discordgo.ApplicationCommandOptionRole:
		id, _ := o.Value.(int64)
		if data.Resolved != nil {
			if r, ok := data.Resolved.Roles[id]; ok {
				return r
			}
		}
		return &discordgo.Role{ID: id}
	case discordgo.ApplicationCommandOptionMentionable:
		id, _ := o.Value.(int64)
		if data.Resolved != nil {
			if r, ok := data.Resolved.Roles[id]; ok {
				return r
			}
		}
		return resolveSlashUser(data, id)
	}

	return o.Value
}

func resolveSlashUser(data *discordgo.ApplicationCommandInteractionData, id int64) *discordgo.User {
	if data.Resolved != nil {
		if u, ok := data.Resolved.Users[id]; ok {
			return u
		}
	}
	return &discordgo.User{ID: id}
}

// buildSlashCommandRequest converts a stored slash command custom command into the
// discordgo request used to (re)register it with Discord.
// buildOptionList converts stored leaf options into discordgo options, sorted
// required-first (Discord rejects a required option after an optional one).
func buildOptionList(options []SlashCommandOption) []*discordgo.ApplicationCommandOption {
	opts := make([]*discordgo.ApplicationCommandOption, 0, len(options))
	for _, o := range options {
		ao := &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionType(o.Type),
			Name:        strings.ToLower(o.Name),
			Description: o.Description,
			Required:    o.Required,
			MinValue:    o.MinValue,
			MinLength:   o.MinLength,
			MaxLength:   o.MaxLength,
		}
		if o.MaxValue != nil {
			ao.MaxValue = *o.MaxValue
		}
		for _, c := range o.Choices {
			// the choice value type must match the option type (string/int/number).
			var value any = c
			switch o.Type {
			case int(discordgo.ApplicationCommandOptionInteger):
				if n, err := strconv.ParseInt(strings.TrimSpace(c), 10, 64); err == nil {
					value = n
				}
			case int(discordgo.ApplicationCommandOptionNumber):
				if f, err := strconv.ParseFloat(strings.TrimSpace(c), 64); err == nil {
					value = f
				}
			}
			ao.Choices = append(ao.Choices, &discordgo.ApplicationCommandOptionChoice{Name: c, Value: value})
		}
		for _, ct := range o.ChannelTypes {
			ao.ChannelTypes = append(ao.ChannelTypes, discordgo.ChannelType(ct))
		}
		opts = append(opts, ao)
	}

	// Discord requires required options to come before optional ones.
	sort.SliceStable(opts, func(i, j int) bool {
		return opts[i].Required && !opts[j].Required
	})
	return opts
}

func buildSlashCommandRequest(cc *models.CustomCommand) *discordgo.CreateApplicationCommandRequest {
	stored := parseSlashCommandData(cc)

	var opts []*discordgo.ApplicationCommandOption
	if len(stored.Subcommands) > 0 {
		// A command that owns subcommands carries one SUB_COMMAND option per
		// subcommand and is not directly invocable. Subcommand ordering is stable;
		// SUB_COMMAND options have no required/optional ordering constraint.
		for _, sub := range stored.Subcommands {
			desc := sub.Description
			if strings.TrimSpace(desc) == "" {
				desc = "Subcommand"
			}
			opts = append(opts, &discordgo.ApplicationCommandOption{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        strings.ToLower(sub.Name),
				Description: common.CutStringShort(desc, MaxSlashCommandDescription),
				Options:     buildOptionList(sub.Options),
			})
		}
	} else {
		opts = buildOptionList(stored.Options)
	}

	description := stored.Description
	if strings.TrimSpace(description) == "" {
		description = "Custom command"
	}

	defaultPermission := true
	return &discordgo.CreateApplicationCommandRequest{
		Name:              strings.ToLower(cc.TextTrigger),
		Description:       common.CutStringShort(description, MaxSlashCommandDescription),
		Options:           opts,
		DefaultPermission: &defaultPermission,
	}
}

// handleResyncGuildSlashCommands is the pubsub handler that rebuilds and pushes a
// guild's slash command set to Discord. It runs on the bot node that owns the
// guild's shard.
func handleResyncGuildSlashCommands(evt *pubsub.Event) {
	guildID := evt.TargetGuildInt
	if guildID == 0 {
		return
	}
	resyncGuildSlashCommands(guildID)
}

func resyncGuildSlashCommands(guildID int64) {
	cmds, err := models.CustomCommands(
		qm.Where("guild_id = ? AND trigger_type = ? AND disabled = false", guildID, int(CommandTriggerSlash)),
		qm.OrderBy("local_id asc"),
	).AllG(context.Background())
	if err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("failed fetching slash command ccs for resync")
		return
	}

	maxSlash := MaxSlashCommandForContext(guildID)
	if len(cmds) > maxSlash {
		logger.WithField("guild", guildID).Warnf("more than %d enabled slash command ccs, only registering the first %d", maxSlash, maxSlash)
		cmds = cmds[:maxSlash]
	}

	set := make([]*discordgo.CreateApplicationCommandRequest, 0, len(cmds))
	for _, cc := range cmds {
		set = append(set, buildSlashCommandRequest(cc))
	}

	// Context menu (USER/MESSAGE) commands are also guild application commands and must
	// be part of the same bulk overwrite, otherwise they'd clobber the slash commands
	// (and vice-versa). They use a separate per-type limit.
	set = append(set, buildGuildContextMenuRequests(guildID)...)

	// Skip the (rate-limited) overwrite if the resulting command set is unchanged.
	serialized, _ := json.Marshal(set)
	hash := sha256.Sum256(serialized)
	redisKey := fmt.Sprintf("slash_cc_cmds_sum:%d", guildID)

	var oldHash []byte
	if err := common.RedisPool.Do(radix.Cmd(&oldHash, "GET", redisKey)); err == nil && bytes.Equal(hash[:], oldHash) {
		return
	}

	_, err = common.BotSession.BulkOverwriteGuildApplicationCommands(common.BotApplication.ID, guildID, set)
	if err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("failed bulk overwriting guild slash commands for custom commands")
		return
	}

	if err := common.RedisPool.Do(radix.Cmd(nil, "SET", redisKey, string(hash[:]))); err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("failed storing slash command cc hash")
	}
}
