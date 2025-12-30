package customcommands

import (
	"context"
	"sort"
	"strings"

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

	// Ephemeral messages always have guild_id = 0 even if created in a guild channel; see
	// https://github.com/discord/discord-api-docs/issues/4557. But exec/execAdmin rely
	// on the guild ID of the message to fill guild data, so patch it here.
	if interaction.Message == nil || interaction.Member == nil {
		return
	}
	interaction.Message.GuildID = evt.GS.ID
	interaction.Member.GuildID = evt.GS.ID

	switch interaction.Type {
	case discordgo.InteractionMessageComponent:
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

		triggeredCmds, err := findComponentOrModalTriggerCustomCommands(evt.Context(), cState, interaction.Member, cID, false)
		if err != nil {
			logger.WithField("guild", evt.GS.ID).WithError(err).Error("failed finding component ccs")
			return
		}

		if len(triggeredCmds) < 1 {
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

		triggeredCmds, err := findComponentOrModalTriggerCustomCommands(evt.Context(), cState, interaction.Member, cID, true)
		if err != nil {
			logger.WithField("guild", evt.GS.ID).WithError(err).Error("failed finding component ccs")
			return
		}

		if len(triggeredCmds) < 1 {
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

func findComponentOrModalTriggerCustomCommands(ctx context.Context, cs *dstate.ChannelState, member *discordgo.Member, cID string, modal bool) (matches []*TriggeredCC, err error) {
	cmds, err := BotCachedGetCommandsWithMessageTriggers(cs.GuildID, ctx)
	if err != nil {
		return nil, errors.WrapIf(err, "BotCachedGetCommandsWithComponentTriggers")
	}

	var matched []*TriggeredCC
	for _, cmd := range cmds {
		if cmd.Disabled || !CmdRunsInChannel(cmd, common.ChannelOrThreadParentID(cs)) || cmd.R.Group != nil && cmd.R.Group.Disabled {
			continue
		}

		if modal {
			if didMatch, stripped, args := CheckMatchModal(cmd, cID); didMatch {

				matched = append(matched, &TriggeredCC{
					CC:       cmd,
					Stripped: stripped,
					Args:     args,
				})
			}
		} else {
			if didMatch, stripped, args := CheckMatchComponent(cmd, cID); didMatch {

				matched = append(matched, &TriggeredCC{
					CC:       cmd,
					Stripped: stripped,
					Args:     args,
				})
			}
		}
	}

	if len(matched) < 1 {
		// no matches
		return matched, nil
	}

	ms, err := bot.GetMember(cs.GuildID, member.User.ID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	if ms.User.Bot {
		return nil, nil
	}

	// filter by roles
	filtered := make([]*TriggeredCC, 0, len(matched))
	for _, v := range matched {
		if !CmdRunsForUser(v.CC, ms) {
			continue
		}

		filtered = append(filtered, v)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return hasHigherPriority(filtered[i].CC, filtered[j].CC)
	})

	limit := CCActionExecLimit(cs.GuildID)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
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
