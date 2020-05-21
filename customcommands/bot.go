package customcommands

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/premium"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/keylock"
	"github.com/jonas747/yagpdb/common/multiratelimit"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	schEventsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var (
	CCExecLock        = keylock.NewKeyLock()
	DelayedCCRunLimit = multiratelimit.NewMultiRatelimiter(0.1, 10)
)

type DelayedRunLimitKey struct {
	GuildID   int64
	ChannelID int64
}

type CCExecKey struct {
	GuildID int64
	CCID    int64
}

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmdListCommands)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleMessageCreate), eventsystem.EventMessageCreate)
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(handleMessageReactions), eventsystem.EventMessageReactionAdd, eventsystem.EventMessageReactionRemove)

	// add the pubsub handler for cache eviction
	pubsub.AddHandler("custom_commands_clear_cache", func(event *pubsub.Event) {
		gs := bot.State.Guild(true, event.TargetGuildInt)
		if gs == nil {
			return
		}

		gs.UserCacheDel(CacheKeyCommands)
	}, nil)

	scheduledevents2.RegisterHandler("cc_next_run", NextRunScheduledEvent{}, handleNextRunScheduledEVent)
	scheduledevents2.RegisterHandler("cc_delayed_run", DelayedRunCCData{}, handleDelayedRunCC)
}

type DelayedRunCCData struct {
	ChannelID int64  `json:"channel_id"`
	CmdID     int64  `json:"cmd_id"`
	UserData  []byte `json:"data"`

	Message *discordgo.Message
	Member  *dstate.MemberState

	UserKey interface{} `json:"user_key"`
}

var cmdListCommands = &commands.YAGCommand{
	CmdCategory:    commands.CategoryTool,
	Name:           "CustomCommands",
	Aliases:        []string{"cc"},
	Description:    "Shows a custom command specified by id or trigger, or lists them all",
	ArgumentCombos: [][]int{[]int{0}, []int{1}, []int{}},
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "ID", Type: dcmd.Int},
		&dcmd.ArgDef{Name: "Trigger", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		ccs, err := models.CustomCommands(qm.Where("guild_id = ?", data.GS.ID), qm.OrderBy("local_id")).AllG(data.Context())
		if err != nil {
			return "Failed retrieving custom commands", err
		}

		groups, err := models.CustomCommandGroups(qm.Where("guild_id=?", data.GS.ID)).AllG(data.Context())
		if err != nil {
			return "Failed retrieving custom command groups", err
		}

		groupMap := make(map[int64]string)
		groupMap[0] = "Ungrouped"
		for _, group := range groups {
			groupMap[group.ID] = group.Name
		}

		foundCCS, provided := FindCommands(ccs, data)
		if len(foundCCS) < 1 {
			list := StringCommands(ccs, groupMap)
			if len(list) == 0 {
				return "This server has no custom commands, sry.", nil
			}
			if provided {
				return "No command by that name or id found, here is a list of them all:\n" + list, nil
			} else {
				return "No id or trigger provided, here is a list of all server commands:\n" + list, nil
			}
		}

		if len(foundCCS) > 1 {
			return "More than 1 matched command\n" + StringCommands(foundCCS, groupMap), nil
		}

		cc := foundCCS[0]

		if cc.TextTrigger != "" {
			return fmt.Sprintf("#%d - %s: `%s` - Case sensitive trigger: `%t` - Group: `%s`\n```\n%s\n```", cc.LocalID, CommandTriggerType(cc.TriggerType), cc.TextTrigger, cc.TextTriggerCaseSensitive, groupMap[cc.GroupID.Int64], strings.Join(cc.Responses, "```\n```")), nil
		} else {
			return fmt.Sprintf("#%d - %s - Group: `%s`\n```\n%s\n```", cc.LocalID, CommandTriggerType(cc.TriggerType), groupMap[cc.GroupID.Int64], strings.Join(cc.Responses, "```\n```")), nil
		}
	},
}

func FindCommands(ccs []*models.CustomCommand, data *dcmd.Data) (foundCCS []*models.CustomCommand, provided bool) {
	foundCCS = make([]*models.CustomCommand, 0, len(ccs))

	provided = true
	if data.Args[0].Value != nil {
		// Find by ID
		id := data.Args[0].Int64()
		for _, v := range ccs {
			if v.LocalID == id {
				foundCCS = append(foundCCS, v)
			}
		}
	} else if data.Args[1].Value != nil {
		// Find by name
		name := data.Args[1].Str()
		for _, v := range ccs {
			if strings.EqualFold(name, v.TextTrigger) {
				foundCCS = append(foundCCS, v)
			}
		}
	} else {
		provided = false
	}

	return
}

func StringCommands(ccs []*models.CustomCommand, gMap map[int64]string) string {
	out := ""
	for _, cc := range ccs {
		switch cc.TextTrigger {
		case "":
			out += fmt.Sprintf("`#%3d:` %s - Group: `%s`\n", cc.LocalID, CommandTriggerType(cc.TriggerType).String(), gMap[cc.GroupID.Int64])
		default:
			out += fmt.Sprintf("`#%3d:` `%s`: %s - Group: `%s`\n", cc.LocalID, cc.TextTrigger, CommandTriggerType(cc.TriggerType).String(), gMap[cc.GroupID.Int64])
		}
	}

	return out
}

func handleDelayedRunCC(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*DelayedRunCCData)
	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", evt.GuildID, dataCast.CmdID)).OneG(context.Background())
	if err != nil {
		return false, errors.WrapIf(err, "find_command")
	}

	if !DelayedCCRunLimit.AllowN(DelayedRunLimitKey{GuildID: evt.GuildID, ChannelID: dataCast.ChannelID}, time.Now(), 1) {
		logger.WithField("guild", cmd.GuildID).Warn("went above delayed cc run ratelimit")
		return false, nil
	}

	gs := bot.State.Guild(true, evt.GuildID)
	if gs == nil {
		// in case the bot left in the meantime
		if onGuild, err := common.BotIsOnGuild(evt.GuildID); !onGuild && err == nil {
			return false, nil
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}

		return true, nil
	}

	cs := gs.Channel(true, dataCast.ChannelID)
	if cs == nil {
		// don't reschedule if channel is deleted, make sure its actually not there, and not just a discord downtime
		if !gs.IsAvailable(true) {
			return true, nil
		}

		return false, nil
	}

	// attempt to get up to date member information
	if dataCast.Member != nil {
		updatedMS, _ := bot.GetMember(gs.ID, dataCast.Member.ID)
		if updatedMS != nil {
			dataCast.Member = updatedMS
		}
	}

	tmplCtx := templates.NewContext(gs, cs, dataCast.Member)
	if dataCast.Message != nil {
		tmplCtx.Msg = dataCast.Message
		tmplCtx.Data["Message"] = dataCast.Message
	}

	// decode userdata
	if len(dataCast.UserData) > 0 {
		var i interface{}
		err := msgpack.Unmarshal(dataCast.UserData, &i)
		if err != nil {
			return false, err
		}

		tmplCtx.Data["ExecData"] = i
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	err = ExecuteCustomCommand(cmd, tmplCtx)
	return false, err
}

func handleNextRunScheduledEVent(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", evt.GuildID, (data.(*NextRunScheduledEvent)).CmdID)).OneG(context.Background())
	if err != nil {
		return false, errors.WrapIf(err, "find_command")
	}

	gs := bot.State.Guild(true, evt.GuildID)
	if gs == nil {
		if onGuild, err := common.BotIsOnGuild(evt.GuildID); !onGuild && err == nil {
			return false, nil
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}

		return true, nil
	}

	cs := gs.Channel(true, cmd.ContextChannel)
	if cs == nil {
		// don't reschedule if channel is deleted, make sure its actually not there, and not just a discord downtime
		if !gs.IsAvailable(true) {
			return true, nil
		}

		return false, nil
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	tmplCtx := templates.NewContext(gs, cs, nil)
	ExecuteCustomCommand(cmd, tmplCtx)

	// schedule next runs
	cmd.LastRun = cmd.NextRun
	err = UpdateCommandNextRunTime(cmd, true)
	if err != nil {
		logger.WithError(err).Error("failed updating custom command next run time")
	}

	return false, nil
}

func shouldIgnoreChannel(evt *discordgo.MessageCreate, cState *dstate.ChannelState) bool {
	if evt.GuildID == 0 {
		return true
	}

	if cState == nil {
		logger.Warn("Channel not found in state")
		return true
	}

	if !bot.IsNormalUserMessage(evt.Message) {
		return true
	}

	if evt.Message.Author.Bot {
		return true
	}

	if !bot.BotProbablyHasPermissionGS(cState.Guild, cState.ID, discordgo.PermissionSendMessages) {
		return true
	}

	// Passed all checks, custom commands should not ignore this channel
	return false
}

const (
	CCMessageExecLimitNormal  = 3
	CCMessageExecLimitPremium = 5
)

var metricsExecutedCommands = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "yagpdb_cc_triggered_total",
	Help: "Number custom commands triggered",
}, []string{"trigger"})

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

	cState := evt.CS()
	if cState == nil {
		return
	}

	if !bot.BotProbablyHasPermissionGS(cState.Guild, cState.ID, discordgo.PermissionSendMessages) {
		// don't run in channel we don't have perms in
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
	rMessage.GuildID = cState.Guild.ID

	for _, matched := range triggeredCmds {
		err = ExecuteCustomCommandFromReaction(matched.CC, ms, cState, reaction, added, rMessage)
		if err != nil {
			logger.WithField("guild", cState.Guild.ID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
		}
	}
}

func ExecuteCustomCommandFromReaction(cc *models.CustomCommand, ms *dstate.MemberState, cs *dstate.ChannelState, reaction *discordgo.MessageReaction, added bool, message *discordgo.Message) error {
	tmplCtx := templates.NewContext(cs.Guild, cs, ms)

	// to make sure the message is in the proper context of the user reacting we set the mssage context to a fake message
	fakeMsg := *message
	fakeMsg.Member = ms.DGoCopy()
	fakeMsg.Author = fakeMsg.Member.User
	tmplCtx.Msg = &fakeMsg

	tmplCtx.Data["Reaction"] = reaction
	tmplCtx.Data["ReactionMessage"] = message
	tmplCtx.Data["ReactionAdded"] = added

	return ExecuteCustomCommand(cc, tmplCtx)
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	mc := evt.MessageCreate()
	cs := evt.CS()

	if !evt.HasFeatureFlag(featureFlagHasCommands) {
		return
	}

	if shouldIgnoreChannel(mc, cs) {
		return
	}

	member := dstate.MSFromDGoMember(evt.GS, mc.Member)
	matchedCustomCommands, err := findMessageTriggerCustomCommands(evt.Context(), cs, member, mc)
	if err != nil {
		logger.WithError(err).Error("Error mathching custom commands")
		return
	}

	if len(matchedCustomCommands) == 0 {
		return
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "message"}).Inc()

	for _, matched := range matchedCustomCommands {
		err = ExecuteCustomCommandFromMessage(matched.CC, member, cs, matched.Args, matched.Stripped, mc.Message)
		if err != nil {
			logger.WithField("guild", mc.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
		}
	}
}

type TriggeredCC struct {
	CC       *models.CustomCommand
	Stripped string
	Args     []string
}

func findMessageTriggerCustomCommands(ctx context.Context, cs *dstate.ChannelState, ms *dstate.MemberState, mc *discordgo.MessageCreate) (matches []*TriggeredCC, err error) {
	cmds, err := BotCachedGetCommandsWithMessageTriggers(cs.Guild, ctx)
	if err != nil {
		return nil, errors.WrapIf(err, "BotCachedGetCommandsWithMessageTriggers")
	}

	prefix, err := commands.GetCommandPrefix(mc.GuildID)
	if err != nil {
		return nil, errors.WrapIf(err, "GetCommandPrefix")
	}

	var matched []*TriggeredCC
	for _, cmd := range cmds {
		if !CmdRunsInChannel(cmd, mc.ChannelID) || !CmdRunsForUser(cmd, ms) {
			continue
		}

		if didMatch, stripped, args := CheckMatch(prefix, cmd, mc.Content); didMatch {

			matched = append(matched, &TriggeredCC{
				CC:       cmd,
				Args:     args,
				Stripped: stripped,
			})
		}
	}

	sortTriggeredCCs(matched)

	limit := CCMessageExecLimitNormal
	if isPremium, _ := premium.IsGuildPremiumCached(mc.GuildID); isPremium {
		limit = CCMessageExecLimitPremium
	}

	if len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, nil
}

func findReactionTriggerCustomCommands(ctx context.Context, cs *dstate.ChannelState, userID int64, reaction *discordgo.MessageReaction, add bool) (ms *dstate.MemberState, matches []*TriggeredCC, err error) {
	cmds, err := BotCachedGetCommandsWithMessageTriggers(cs.Guild, ctx)
	if err != nil {
		return nil, nil, errors.WrapIf(err, "BotCachedGetCommandsWithMessageTriggers")
	}

	var matched []*TriggeredCC
	for _, cmd := range cmds {
		if !CmdRunsInChannel(cmd, reaction.ChannelID) {
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

	ms, err = bot.GetMember(cs.Guild.ID, userID)
	if err != nil {
		return nil, nil, errors.WithStackIf(err)
	}

	// filter by roles
	filtered := make([]*TriggeredCC, 0, len(matched))
	for _, v := range matched {
		if !CmdRunsForUser(v.CC, ms) {
			continue
		}

		filtered = append(filtered, v)
	}

	sortTriggeredCCs(filtered)

	limit := CCMessageExecLimitNormal
	if isPremium, _ := premium.IsGuildPremiumCached(cs.Guild.ID); isPremium {
		limit = CCMessageExecLimitPremium
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return ms, filtered, nil
}

func sortTriggeredCCs(ccs []*TriggeredCC) {
	sort.Slice(ccs, func(i, j int) bool {
		a := ccs[i]
		b := ccs[j]

		if a.CC.TriggerType == b.CC.TriggerType {
			return a.CC.LocalID < b.CC.LocalID
		}

		if a.CC.TriggerType == int(CommandTriggerRegex) {
			return false
		}

		if b.CC.TriggerType == int(CommandTriggerRegex) {
			return true
		}

		return a.CC.LocalID < b.CC.LocalID
	})
}

func ExecuteCustomCommandFromMessage(cmd *models.CustomCommand, member *dstate.MemberState, cs *dstate.ChannelState, cmdArgs []string, stripped string, m *discordgo.Message) error {
	tmplCtx := templates.NewContext(cs.Guild, cs, member)
	tmplCtx.Msg = m

	// preapre message specific data
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
	tmplCtx.Data["Message"] = m

	return ExecuteCustomCommand(cmd, tmplCtx)
}

// func ExecuteCustomCommand(cmd *models.CustomCommand, cmdArgs []string, stripped string, s *discordgo.Session, m *discordgo.MessageCreate) (resp string, tmplCtx *templates.Context, err error) {
func ExecuteCustomCommand(cmd *models.CustomCommand, tmplCtx *templates.Context) error {
	defer func() {
		if err := recover(); err != nil {
			actualErr := ""
			switch t := err.(type) {
			case error:
				actualErr = t.Error()
			case string:
				actualErr = t
			}
			onExecPanic(cmd, errors.New(actualErr), tmplCtx, true)
		}
	}()

	tmplCtx.Name = "CC #" + strconv.Itoa(int(cmd.LocalID))
	tmplCtx.Data["CCID"] = cmd.LocalID
	tmplCtx.Data["CCRunCount"] = cmd.RunCount + 1

	csCop := tmplCtx.CurrentFrame.CS.Copy(true)
	f := logger.WithFields(logrus.Fields{
		"trigger":      cmd.TextTrigger,
		"trigger_type": CommandTriggerType(cmd.TriggerType).String(),
		"guild":        csCop.Guild.ID,
		"channel_name": csCop.Name,
	})

	// do not allow concurrect executions of the same custom command, to prevent most common kinds of abuse
	lockKey := CCExecKey{
		GuildID: cmd.GuildID,
		CCID:    cmd.LocalID,
	}
	lockHandle := CCExecLock.Lock(lockKey, time.Minute, time.Minute*10)
	if lockHandle == -1 {
		f.Warn("Exceeded max lock attempts for cc")
		if cmd.ShowErrors {
			common.BotSession.ChannelMessageSend(tmplCtx.CurrentFrame.CS.ID, fmt.Sprintf("Gave up trying to execute custom command #%d after 1 minute because there is already one or more instances of it being executed.", cmd.LocalID))
		}
		updatePostCommandRan(cmd, errors.New("Gave up trying to execute, already an existing instance executing"))
		return nil
	}

	defer CCExecLock.Unlock(lockKey, lockHandle)

	go analytics.RecordActiveUnit(cmd.GuildID, &Plugin{}, "executed_cc")

	// pick a response and execute it
	f.Info("Custom command triggered")

	chanMsg := cmd.Responses[rand.Intn(len(cmd.Responses))]
	out, err := tmplCtx.Execute(chanMsg)

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command response was longer than 2k (contact an admin on the server...)"
	}

	go updatePostCommandRan(cmd, err)

	// deal with the results
	if err != nil {
		logger.WithField("guild", tmplCtx.GS.ID).WithError(err).Error("Error executing custom command")
		if cmd.ShowErrors {
			out += "\nAn error caused the execution of the custom command template to stop:\n"
			out += "`" + err.Error() + "`"
		}
	}

	_, err = tmplCtx.SendResponse(out)
	if err != nil {
		return errors.WithStackIf(err)
	}
	return nil
}

func onExecPanic(cmd *models.CustomCommand, err error, tmplCtx *templates.Context, logStack bool) {
	l := logger.WithField("guild", tmplCtx.GS.ID).WithError(err)
	if logStack {
		stack := string(debug.Stack())
		l = l.WithField("stack", stack)
	}

	l.Error("Error executing custom command")

	if cmd.ShowErrors {
		out := "\nAn error caused the execution of the custom command template to stop:\n"
		out += "`" + err.Error() + "`"

		common.BotSession.ChannelMessageSend(tmplCtx.CurrentFrame.CS.ID, out)
	}

	updatePostCommandRan(cmd, err)
}

func updatePostCommandRan(cmd *models.CustomCommand, runErr error) {
	const qNoErr = "UPDATE custom_commands SET run_count = run_count + 1 WHERE guild_id=$1 AND local_id=$2"
	const qErr = "UPDATE custom_commands SET run_count = run_count + 1, last_error=$3, last_error_time=now() WHERE guild_id=$1 AND local_id=$2"

	var err error
	if runErr == nil {
		_, err = common.PQ.Exec(qNoErr, cmd.GuildID, cmd.LocalID)
	} else {
		_, err = common.PQ.Exec(qErr, cmd.GuildID, cmd.LocalID, runErr.Error())
	}

	if err != nil {
		logger.WithError(err).WithField("guild", cmd.GuildID).Error("failed running post command executed query")
	}

	// if runErr != nil {
	// 	err := pubsub.Publish("custom_commands_clear_cache", cmd.GuildID, nil)
	// 	if err != nil {
	// 		logger.WithError(err).Error("failed creating cache eviction pubsub event in updatePostCommandRan")
	// 	}
	// }
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
		cmdMatch += `\A.*` + regexp.QuoteMeta(trigger)
	case CommandTriggerRegex:
		cmdMatch += trigger
	case CommandTriggerExact:
		cmdMatch += `\A` + regexp.QuoteMeta(trigger) + `\z`
	default:
		return false, "", nil
	}

	item, err := RegexCache.Fetch(cmdMatch, time.Minute*10, func() (interface{}, error) {
		re, err := regexp.Compile(cmdMatch)
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

	// The following simply matches the legacy behavior as I'm not sure if anyone is relying on it.
	if !cmd.TextTriggerCaseSensitive && cmd.TriggerType != int(CommandTriggerRegex) {
		stripped = strings.ToLower(msg)
	}

	stripped = msg[idx[1]:]
	match = true
	return
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

type CacheKey int

const (
	CacheKeyCommands CacheKey = iota
	CacheKeyReactionCommands

	CacheKeyDBLimits
)

func BotCachedGetCommandsWithMessageTriggers(gs *dstate.GuildState, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := gs.UserCacheFetch(CacheKeyCommands, func() (interface{}, error) {
		return models.CustomCommands(qm.Where("guild_id = ? AND trigger_type IN (0,1,2,3,4,6)", gs.Guild.ID), qm.OrderBy("local_id desc"), qm.Load("Group")).AllG(ctx)
	})

	if err != nil {
		return nil, err
	}

	return v.(models.CustomCommandSlice), nil
}
