package customcommands

import (
	"context"
	"sort"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func handleInteractionCreate(evt *eventsystem.EventData) {
	i := evt.EvtInterface.(*discordgo.InteractionCreate).Interaction
	interaction := templates.CustomCommandInteraction{Interaction: &i, RespondedTo: false}

	if interaction.GuildID == 0 {
		// ignore dm interactions
		return
	}

	evt.GS = bot.State.GetGuild(interaction.GuildID)
	if evt.GS == nil {
		logrus.WithField("guild_id", interaction.GuildID).Error("Couldn't get Guild from state for interaction create")
		return
	}

	evt.GuildFeatureFlags, _ = featureflags.RetryGetGuildFlags(evt.GS.ID)
	if !evt.HasFeatureFlag(featureFlagHasCommands) {
		return
	}

	cState := evt.GS.GetChannelOrThread(interaction.ChannelID)
	if cState == nil {
		return
	}

	// Slash command interactions carry no source message, so handle them before the
	// message/member guard below that the component & modal paths rely on.
	if interaction.Type == discordgo.InteractionApplicationCommand {
		handleSlashCommandInteraction(evt, cState, &interaction)
		return
	}

	// A guild interaction always has a member. Components are always attached to a
	// message, but modals opened from a slash command have no source message, so the
	// message is only required by (and patched for) the component path below.
	if interaction.Member == nil {
		return
	}

	// Ephemeral messages always have guild_id = 0 even if created in a guild channel;
	// see https://github.com/discord/discord-api-docs/issues/4557. But exec/execAdmin
	// rely on the guild ID of the message to fill guild data, so patch it here.
	interaction.Member.GuildID = evt.GS.ID
	if interaction.Message != nil {
		interaction.Message.GuildID = evt.GS.ID
	}

	switch interaction.Type {
	case discordgo.InteractionMessageComponent:
		if interaction.Message == nil {
			return
		}

		cMessage, err := common.BotSession.ChannelMessage(cState.ID, interaction.Message.ID)
		if err == nil {
			cMessage.GuildID = cState.GuildID
			interaction.Message = cMessage
		}

		cID := interaction.MessageComponentData().CustomID

		// continue only if this component was created by a cc
		cID, ok := strings.CutPrefix(cID, templates.TemplateCustomIDPrefix)
		if !ok {
			return
		}

		triggeredCmds, restricted, err := findComponentOrModalTriggerCustomCommands(evt.Context(), cState, interaction.Member, cID, false)
		if err != nil {
			logger.WithField("guild", evt.GS.ID).WithError(err).Error("failed finding component ccs")
			return
		}

		if len(triggeredCmds) < 1 {
			if restricted {
				respondRestrictedInteraction(&interaction)
			}
			return
		}

		deferResponseToCCs(&interaction, triggeredCmds)
		for _, matched := range triggeredCmds {
			err = ExecuteCustomCommandFromComponent(matched.CC, evt.GS, cState, matched.Args, matched.Stripped, &interaction)
			if err != nil {
				logger.WithField("guild", cState.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
			}
		}
	case discordgo.InteractionModalSubmit:
		cID := interaction.ModalSubmitData().CustomID

		// continue only if this modal was created by a cc
		cID, ok := strings.CutPrefix(cID, templates.TemplateCustomIDPrefix)
		if !ok {
			return
		}

		triggeredCmds, restricted, err := findComponentOrModalTriggerCustomCommands(evt.Context(), cState, interaction.Member, cID, true)
		if err != nil {
			logger.WithField("guild", evt.GS.ID).WithError(err).Error("failed finding component ccs")
			return
		}

		if len(triggeredCmds) < 1 {
			if restricted {
				respondRestrictedInteraction(&interaction)
			}
			return
		}

		deferResponseToCCs(&interaction, triggeredCmds)
		for _, matched := range triggeredCmds {
			err = ExecuteCustomCommandFromModal(matched.CC, evt.GS, cState, matched.Args, matched.Stripped, &interaction)
			if err != nil {
				logger.WithField("guild", cState.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
			}
		}
	}
}

func respondRestrictedInteraction(interaction *templates.CustomCommandInteraction) {
	if interaction.RespondedTo {
		return
	}

	err := common.BotSession.CreateInteractionResponse(interaction.ID, interaction.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: "This command is restricted and can't be used in this channel or by you.",
		},
	})
	if err != nil {
		logger.WithField("guild", interaction.GuildID).WithError(err).Error("Error responding to restricted interaction")
		return
	}
	interaction.RespondedTo = true
}

func deferResponseToCCs(interaction *templates.CustomCommandInteraction, ccs []*TriggeredCC) {
	def := &discordgo.InteractionResponse{
		Data: &discordgo.InteractionResponseData{},
	}

	for _, c := range ccs {
		switch c.CC.InteractionDeferMode {
		case InteractionDeferModeNone:
			continue
		case InteractionDeferModeMessage:
			def.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
		case InteractionDeferModeEphemeral:
			def.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
			def.Data.Flags = def.Data.Flags | discordgo.MessageFlagsEphemeral
		case InteractionDeferModeUpdate:
			def.Type = discordgo.InteractionResponseDeferredMessageUpdate
		}

		break
	}

	if def.Type != 0 {
		err := common.BotSession.CreateInteractionResponse(interaction.ID, interaction.Token, def)
		if err != nil {
			logger.WithField("guild", interaction.GuildID).WithError(err).Error("Error deferring response")
		}
		interaction.RespondedTo = true
		interaction.Deferred = true
	}
}

var cachedCommandsComponentTrigger = common.CacheSet.RegisterSlot("custom_commands_component_trigger", nil, int64(0))

func BotCachedGetCommandsWithComponentTrigger(guildID int64, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := cachedCommandsComponentTrigger.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		var cmds []*models.CustomCommand
		var err error
		common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands from db", logrus.Fields{"guild": guildID}, func() {
			cmds, err = models.CustomCommands(qm.Where("guild_id = ? AND trigger_type IN (7,8)", guildID), qm.OrderBy("local_id desc"), qm.Load("Group")).AllG(ctx)
		})
		return cmds, err
	})
	if err != nil {
		return nil, err
	}
	return v.([]*models.CustomCommand), nil
}

func findComponentOrModalTriggerCustomCommands(ctx context.Context, cs *dstate.ChannelState, member *discordgo.Member, cID string, modal bool) (matches []*TriggeredCC, restricted bool, err error) {
	cmds, err := BotCachedGetCommandsWithComponentTrigger(cs.GuildID, ctx)
	if err != nil {
		return nil, false, errors.WrapIf(err, "BotCachedGetCommandsWithComponentTriggers")
	}

	var triggerMatched []*TriggeredCC
	for _, cmd := range cmds {
		if cmd.Disabled || (cmd.R != nil && cmd.R.Group != nil && cmd.R.Group.Disabled) {
			continue
		}

		var didMatch bool
		var stripped string
		var args []string
		if modal {
			didMatch, stripped, args = CheckMatchModal(cmd, cID)
		} else {
			didMatch, stripped, args = CheckMatchComponent(cmd, cID)
		}
		if didMatch {
			triggerMatched = append(triggerMatched, &TriggeredCC{CC: cmd, Stripped: stripped, Args: args})
		}
	}

	if len(triggerMatched) < 1 {
		// no matches
		return nil, false, nil
	}

	ms, err := bot.GetMember(cs.GuildID, member.User.ID)
	if err != nil {
		return nil, false, errors.WithStackIf(err)
	}

	if ms.User.Bot {
		return nil, false, nil
	}

	// filter by channel and role restrictions
	filtered := make([]*TriggeredCC, 0, len(triggerMatched))
	for _, v := range triggerMatched {
		if !CmdRunsInChannel(v.CC, common.ChannelOrThreadParentID(cs)) || !CmdRunsForUser(v.CC, ms) {
			continue
		}
		filtered = append(filtered, v)
	}

	if len(filtered) < 1 {
		return nil, true, nil
	}

	sort.Slice(filtered, func(i, j int) bool {
		return hasHigherPriority(filtered[i].CC, filtered[j].CC)
	})

	limit := CCActionExecLimit(cs.GuildID)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, false, nil
}

func ExecuteCustomCommandFromComponent(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, cmdArgs []string, stripped string, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, ms)
	tmplCtx.CurrentFrame.Interaction = interaction

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = interaction.MessageComponentData()
	cid := strings.TrimPrefix(interaction.MessageComponentData().CustomID, templates.TemplateCustomIDPrefix)
	tmplCtx.Data["CustomID"] = cid
	tmplCtx.Data["Cmd"] = cmdArgs[0]
	if len(cmdArgs) > 1 {
		tmplCtx.Data["CmdArgs"] = cmdArgs[1:]
	} else {
		tmplCtx.Data["CmdArgs"] = []string{}
	}
	tmplCtx.Data["StrippedID"] = stripped
	tmplCtx.Data["StrippedMsg"] = stripped

	switch interaction.MessageComponentData().ComponentType {
	case discordgo.ButtonComponent:
		tmplCtx.Data["IsButton"] = true
	case discordgo.SelectMenuComponent, discordgo.UserSelectMenuComponent, discordgo.RoleSelectMenuComponent, discordgo.MentionableSelectMenuComponent, discordgo.ChannelSelectMenuComponent:
		tmplCtx.Data["IsMenu"] = true
		switch interaction.MessageComponentData().ComponentType {
		case discordgo.SelectMenuComponent:
			tmplCtx.Data["MenuType"] = "string"
		case discordgo.UserSelectMenuComponent:
			tmplCtx.Data["MenuType"] = "user"
		case discordgo.RoleSelectMenuComponent:
			tmplCtx.Data["MenuType"] = "role"
		case discordgo.MentionableSelectMenuComponent:
			tmplCtx.Data["MenuType"] = "mentionable"
		case discordgo.ChannelSelectMenuComponent:
			tmplCtx.Data["MenuType"] = "channel"
		}
		tmplCtx.Data["Values"] = interaction.MessageComponentData().Values
	}

	msg := interaction.Message
	msg.Member = ms.DgoMember()
	msg.Author = msg.Member.User
	tmplCtx.Msg = msg

	tmplCtx.Data["Message"] = msg

	return ExecuteCustomCommand(cc, tmplCtx)
}

func CheckMatchComponent(cmd *models.CustomCommand, cID string) (match bool, stripped string, args []string) {

	if cmd.TriggerType != int(CommandTriggerComponent) {
		return false, "", nil
	}

	cmdMatch := "(?m)"
	if !cmd.TextTriggerCaseSensitive {
		cmdMatch += "(?i)"
	}
	cmdMatch += cmd.TextTrigger

	match, stripped, args = matchRegexSplitArgs(cmdMatch, cID)
	return
}

func ExecuteCustomCommandFromModal(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, cmdArgs []string, stripped string, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, ms)
	tmplCtx.CurrentFrame.Interaction = interaction

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = interaction.ModalSubmitData()
	modalCustomID := strings.TrimPrefix(interaction.ModalSubmitData().CustomID, templates.TemplateCustomIDPrefix)
	tmplCtx.Data["CustomID"] = modalCustomID
	tmplCtx.Data["Cmd"] = cmdArgs[0]
	if len(cmdArgs) > 1 {
		tmplCtx.Data["CmdArgs"] = cmdArgs[1:]
	} else {
		tmplCtx.Data["CmdArgs"] = []string{}
	}
	tmplCtx.Data["StrippedID"] = stripped
	tmplCtx.Data["StrippedMsg"] = stripped
	tmplCtx.Data["IsModal"] = true
	cmdValues := []any{}

	modalValues := templates.SDict{}
	for i := 0; i < len(interaction.ModalSubmitData().Components); i++ {
		switch comp := interaction.ModalSubmitData().Components[i].(type) {
		case *discordgo.ActionsRow:
			for j := 0; j < len(comp.Components); j++ {
				field, ok := comp.Components[j].(*discordgo.TextInput)
				if ok {
					cmdValues = append(cmdValues, field.Value)
				}
				cID, _ := strings.CutPrefix(field.CustomID, templates.TemplateCustomIDPrefix)
				modalValues.Set(cID, templates.SDict{
					"type":      field.Type(),
					"value":     field.Value,
					"custom_id": cID,
				})
			}
		case *discordgo.Label:
			switch comp.Component.(type) {
			case *discordgo.TextInput:
				t, _ := comp.Component.(*discordgo.TextInput)
				cID, _ := strings.CutPrefix(t.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, t.Value)
				modalValues.Set(cID, templates.SDict{
					"type":      t.Type(),
					"value":     t.Value,
					"custom_id": cID,
				})
			case *discordgo.Checkbox:
				cb, _ := comp.Component.(*discordgo.Checkbox)
				cID, _ := strings.CutPrefix(cb.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, cb.Value)
				modalValues.Set(cID, templates.SDict{
					"type":      cb.Type(),
					"value":     cb.Value,
					"custom_id": cID,
				})
			case *discordgo.RadioGroup:
				rg, _ := comp.Component.(*discordgo.RadioGroup)
				cID, _ := strings.CutPrefix(rg.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, rg.Value)
				modalValues.Set(cID, templates.SDict{
					"type":      rg.Type(),
					"value":     rg.Value,
					"custom_id": cID,
				})
			case *discordgo.SelectMenu:
				sm, _ := comp.Component.(*discordgo.SelectMenu)
				cID, _ := strings.CutPrefix(sm.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, sm.Values)
				modalValues.Set(cID, templates.SDict{
					"type":      sm.Type(),
					"value":     sm.Values,
					"custom_id": cID,
				})
			case *discordgo.CheckboxGroup:
				cbg, _ := comp.Component.(*discordgo.CheckboxGroup)
				cID, _ := strings.CutPrefix(cbg.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, cbg.Values)
				modalValues.Set(cID, templates.SDict{
					"type":      cbg.Type(),
					"value":     cbg.Values,
					"custom_id": cID,
				})
			}
		}
	}
	tmplCtx.Data["Values"] = cmdValues
	tmplCtx.Data["ModalValues"] = modalValues

	msg := interaction.Message
	if msg == nil {
		msg = &discordgo.Message{GuildID: gs.ID, ChannelID: cs.ID}
	}
	msg.Member = ms.DgoMember()
	msg.Author = msg.Member.User
	tmplCtx.Msg = msg

	tmplCtx.Data["Message"] = msg

	return ExecuteCustomCommand(cc, tmplCtx)
}

func CheckMatchModal(cmd *models.CustomCommand, cID string) (match bool, stripped string, args []string) {

	if cmd.TriggerType != int(CommandTriggerModal) {
		return false, "", nil
	}

	cmdMatch := "(?m)"
	if !cmd.TextTriggerCaseSensitive {
		cmdMatch += "(?i)"
	}
	cmdMatch += cmd.TextTrigger

	match, stripped, args = matchRegexSplitArgs(cmdMatch, cID)
	return
}
