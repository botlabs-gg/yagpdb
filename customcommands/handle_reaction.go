package customcommands

import (
	"context"
	"sort"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/prometheus/client_golang/prometheus"
)

func handleMessageReactions(evt *eventsystem.EventData) {
	var reaction *discordgo.MessageReaction
	var added bool

	switch e := evt.EvtInterface.(type) {
	case *discordgo.MessageReactionAdd:
		added = true
		reaction = e.MessageReaction
	case *discordgo.MessageReactionRemove:
		reaction = e.MessageReaction
	}

	if reaction.GuildID == 0 || reaction.UserID == common.BotUser.ID {
		// ignore dm reactions and reactions from the bot
		return
	}

	if !evt.HasFeatureFlag(featureFlagHasCommands) {
		return
	}

	cState := evt.CSOrThread()
	if cState == nil {
		return
	}
	// if the execution channel is a thread, check for send message in thread perms on the parent channel.
	permToCheck := discordgo.PermissionSendMessages
	cID := cState.ID
	if cState.Type.IsThread() {
		permToCheck = discordgo.PermissionSendMessagesInThreads
		cID = cState.ParentID
	}

	if hasPerms, _ := bot.BotHasPermissionGS(evt.GS, cID, permToCheck); !hasPerms {
		// don't run in channel or thread we don't have perms in
		return
	}

	ms, triggeredCmds, err := findReactionTriggerCustomCommands(evt.Context(), cState, reaction.UserID, reaction, added)
	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeUnknownMember) {
			// example scenario: removing reactions of a user that's not on the server
			// (reactions aren't cleared automatically when a user leaves)
			return
		}

		logger.WithField("guild", evt.GS.ID).WithError(err).Error("failed finding reaction ccs")
		return
	}

	if len(triggeredCmds) < 1 {
		return
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "reaction"}).Inc()

	rMessage, err := common.BotSession.ChannelMessage(cState.ID, reaction.MessageID)
	if err != nil {
		logger.WithField("guild", evt.GS.ID).WithError(err).Error("failed finding reaction ccs")
		return
	}
	rMessage.GuildID = cState.GuildID

	for _, matched := range triggeredCmds {
		err = ExecuteCustomCommandFromReaction(matched.CC, evt.GS, ms, cState, reaction, added, rMessage)
		if err != nil {
			logger.WithField("guild", cState.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
		}
	}
}

func findReactionTriggerCustomCommands(ctx context.Context, cs *dstate.ChannelState, userID int64, reaction *discordgo.MessageReaction, add bool) (ms *dstate.MemberState, matches []*TriggeredCC, err error) {
	cmds, err := BotCachedGetCommandsWithMessageTriggers(cs.GuildID, ctx)
	if err != nil {
		return nil, nil, errors.WrapIf(err, "BotCachedGetCommandsWithReactionTriggers")
	}

	var matched []*TriggeredCC
	for _, cmd := range cmds {
		if cmd.Disabled || !CmdRunsInChannel(cmd, common.ChannelOrThreadParentID(cs)) || cmd.R.Group != nil && cmd.R.Group.Disabled {
			continue
		}

		if didMatch := CheckMatchReaction(cmd, reaction, add); didMatch {

			matched = append(matched, &TriggeredCC{
				CC: cmd,
			})
		}
	}

	if len(matched) < 1 {
		// no matches
		return nil, matched, nil
	}

	ms, err = bot.GetMember(cs.GuildID, userID)
	if err != nil {
		return nil, nil, errors.WithStackIf(err)
	}

	if ms.User.Bot {
		return nil, nil, nil
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

	return ms, filtered, nil
}

func ExecuteCustomCommandFromReaction(cc *models.CustomCommand, gs *dstate.GuildSet, ms *dstate.MemberState, cs *dstate.ChannelState, reaction *discordgo.MessageReaction, added bool, message *discordgo.Message) error {
	tmplCtx := templates.NewContext(gs, cs, ms)

	// to make sure the message is in the proper context of the user reacting we set the mssage context to a fake message
	fakeMsg := *message
	fakeMsg.Member = ms.DgoMember()
	fakeMsg.Author = fakeMsg.Member.User
	tmplCtx.Msg = &fakeMsg

	tmplCtx.Data["Reaction"] = reaction
	tmplCtx.Data["ReactionMessage"] = message
	tmplCtx.Data["Message"] = message
	tmplCtx.Data["ReactionAdded"] = added

	return ExecuteCustomCommand(cc, tmplCtx)
}

func CheckMatchReaction(cmd *models.CustomCommand, reaction *discordgo.MessageReaction, add bool) (match bool) {
	if cmd.TriggerType != int(CommandTriggerReaction) {
		return false
	}

	switch cmd.ReactionTriggerMode {
	case ReactionModeBoth:
		return true
	case ReactionModeAddOnly:
		return add
	case ReactionModeRemoveOnly:
		return !add
	}

	return false
}
