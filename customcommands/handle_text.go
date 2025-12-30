package customcommands

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// hasHigherPriority reports whether custom command a should be executed in preference to custom
// command b. Regex custom commands always have lowest priority, with ties broken by ID (smaller ID
// has priority.)
func hasHigherPriority(a *models.CustomCommand, b *models.CustomCommand) bool {
	aIsRegex := a.TriggerType == int(CommandTriggerRegex)
	bIsRegex := b.TriggerType == int(CommandTriggerRegex)

	switch {
	case !aIsRegex && bIsRegex:
		return true
	case aIsRegex && !bIsRegex:
		return false
	default:
		return a.LocalID < b.LocalID
	}
}

func findMessageTriggerCustomCommands(ctx context.Context, cs *dstate.ChannelState, ms *dstate.MemberState, evt *eventsystem.EventData, msg *discordgo.Message) (matches []*TriggeredCC, err error) {
	cmds, err := BotCachedGetCommandsWithMessageTriggers(cs.GuildID, ctx)
	if err != nil {
		return nil, errors.WrapIf(err, "BotCachedGetCommandsWithMessageTriggers")
	}

	prefix, err := commands.GetCommandPrefixBotEvt(evt)
	if err != nil {
		return nil, errors.WrapIf(err, "GetCommandPrefix")
	}

	var matched []*TriggeredCC
	for _, cmd := range cmds {
		if cmd.Disabled || !CmdRunsInChannel(cmd, common.ChannelOrThreadParentID(cs)) || !CmdRunsForUser(cmd, ms) || cmd.R.Group != nil && cmd.R.Group.Disabled {
			continue
		}
		content := msg.Content
		if cmd.TriggerType == int(CommandTriggerContains) || cmd.TriggerType == int(CommandTriggerRegex) {
			//for contains and regex match, we need to look at the content of the forwarded message too.
			content = strings.Join(msg.GetMessageContents(), " ")
		}
		if didMatch, stripped, args := CheckMatch(prefix, cmd, content); didMatch {
			matched = append(matched, &TriggeredCC{
				CC:       cmd,
				Args:     args,
				Stripped: stripped,
			})
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return hasHigherPriority(matched[i].CC, matched[j].CC)
	})

	limit := CCActionExecLimit(msg.GuildID)
	if len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, nil
}

func HandleMessageUpdate(evt *eventsystem.EventData) {
	mu := evt.MessageUpdate()
	cs := evt.CSOrThread()

	if isPremium, _ := premium.IsGuildPremium(mu.GuildID); !isPremium {
		return
	}

	if !evt.HasFeatureFlag(featureFlagHasCommands) {
		return
	}

	if shouldIgnoreChannel(mu.Message, evt.GS, cs) {
		return
	}

	member := dstate.MemberStateFromMember(mu.Member)
	member.GuildID = evt.GS.ID
	var matchedCustomCommands []*TriggeredCC
	var err error
	common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands", logrus.Fields{"guild": evt.GS.ID}, func() {
		matchedCustomCommands, err = findMessageTriggerCustomCommands(evt.Context(), cs, member, evt, mu.Message)
	})
	if err != nil {
		logger.WithError(err).Error("Error matching custom commands")
		return
	}

	if len(matchedCustomCommands) == 0 {
		return
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "message"}).Inc()
	for _, matched := range matchedCustomCommands {
		if !matched.CC.TriggerOnEdit {
			continue
		}
		err = ExecuteCustomCommandFromMessage(evt.GS, matched.CC, member, cs, matched.Args, matched.Stripped, mu.Message, true)
		if err != nil {
			logger.WithField("guild", mu.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
		}
	}
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	mc := evt.MessageCreate()
	cs := evt.CSOrThread()

	if !evt.HasFeatureFlag(featureFlagHasCommands) {
		return
	}

	if shouldIgnoreChannel(mc.Message, evt.GS, cs) {
		return
	}

	member := dstate.MemberStateFromMember(mc.Member)
	member.GuildID = evt.GS.ID

	var matchedCustomCommands []*TriggeredCC
	var err error
	common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands", logrus.Fields{"guild": evt.GS.ID}, func() {
		matchedCustomCommands, err = findMessageTriggerCustomCommands(evt.Context(), cs, member, evt, mc.Message)
	})
	if err != nil {
		logger.WithError(err).Error("Error matching custom commands")
		return
	}

	if len(matchedCustomCommands) == 0 {
		return
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "message"}).Inc()

	for _, matched := range matchedCustomCommands {
		err = ExecuteCustomCommandFromMessage(evt.GS, matched.CC, member, cs, matched.Args, matched.Stripped, mc.Message, false)
		if err != nil {
			logger.WithField("guild", mc.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
		}
	}
}

// CheckMatch returns true if the given cmd matches, as well as the arguments
// following the command trigger (arg 0 being the message up to, and including,
// the trigger).
func CheckMatch(globalPrefix string, cmd *models.CustomCommand, msg string) (match bool, stripped string, args []string) {
	trigger := cmd.TextTrigger

	cmdMatch := "(?m)"
	if !cmd.TextTriggerCaseSensitive {
		cmdMatch += "(?i)"
	}

	switch CommandTriggerType(cmd.TriggerType) {
	case CommandTriggerCommand:
		// Regex is:
		// \A(<@!?bot_id> ?|server_cmd_prefix)trigger(\z|[[:space:]])
		cmdMatch += `\A(<@!?` + discordgo.StrID(common.BotUser.ID) + "> ?|" + regexp.QuoteMeta(globalPrefix) + ")" + regexp.QuoteMeta(trigger) + `(\z|[[:space:]])`
	case CommandTriggerStartsWith:
		cmdMatch += `\A` + regexp.QuoteMeta(trigger)
	case CommandTriggerContains:
		cmdMatch += regexp.QuoteMeta(trigger)
	case CommandTriggerRegex:
		cmdMatch += trigger
	case CommandTriggerExact:
		cmdMatch += `\A` + regexp.QuoteMeta(trigger) + `\z`
	default:
		return false, "", nil
	}

	return matchRegexSplitArgs(cmdMatch, msg)
}

func ExecuteCustomCommandFromMessage(gs *dstate.GuildSet, cmd *models.CustomCommand, member *dstate.MemberState, cs *dstate.ChannelState, cmdArgs []string, stripped string, m *discordgo.Message, isEdit bool) error {
	tmplCtx := templates.NewContext(gs, cs, member)
	tmplCtx.Msg = m

	// prepare message specific data
	args := dcmd.SplitArgs(m.Content)
	argsStr := make([]string, len(args))
	for k, v := range args {
		argsStr[k] = v.Str
	}

	tmplCtx.Data["Args"] = argsStr
	tmplCtx.Data["StrippedMsg"] = stripped
	tmplCtx.Data["Cmd"] = cmdArgs[0]
	if len(cmdArgs) > 1 {
		tmplCtx.Data["CmdArgs"] = cmdArgs[1:]
	} else {
		tmplCtx.Data["CmdArgs"] = []string{}
	}
	tmplCtx.Data["IsMessageEdit"] = isEdit
	tmplCtx.Data["Message"] = m

	return ExecuteCustomCommand(cmd, tmplCtx)
}

func matchRegexSplitArgs(pattern, msg string) (match bool, stripped string, args []string) {
	item, err := RegexCache.Fetch(pattern, time.Minute*10, func() (interface{}, error) {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}

		return re, nil
	})

	if err != nil {
		return false, "", nil
	}

	re := item.Value().(*regexp.Regexp)

	idx := re.FindStringIndex(msg)
	if idx == nil {
		return false, "", nil
	}

	argsRaw := dcmd.SplitArgs(msg[idx[1]:])
	args = make([]string, len(argsRaw)+1)
	args[0] = strings.TrimSpace(msg[:idx[1]])
	for i, v := range argsRaw {
		args[i+1] = v.Str
	}

	stripped = msg[idx[1]:]
	match = true
	return
}

var cachedCommandsMessage = common.CacheSet.RegisterSlot("custom_commands_message_trigger", nil, int64(0))

func BotCachedGetCommandsWithMessageTriggers(guildID int64, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := cachedCommandsMessage.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		var cmds []*models.CustomCommand
		var err error

		common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands from db", logrus.Fields{"guild": guildID}, func() {
			cmds, err = models.CustomCommands(qm.Where("guild_id = ? AND trigger_type IN (0,1,2,3,4,6,7,8,9)", guildID), qm.OrderBy("local_id desc"), qm.Load("Group")).AllG(ctx)
		})

		return cmds, err
	})

	if err != nil {
		return nil, err
	}

	return v.([]*models.CustomCommand), nil
}
