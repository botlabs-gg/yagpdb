package customcommands

import (
	"bytes"
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

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/lib/template"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/keylock"
	"github.com/botlabs-gg/yagpdb/v2/common/multiratelimit"
	prfx "github.com/botlabs-gg/yagpdb/v2/common/prefix"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	schEventsModels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var (
	CCExecLock        = keylock.NewKeyLock[CCExecKey]()
	DelayedCCRunLimit = multiratelimit.NewMultiRatelimiter(0.1, 10)
	CCMaxDataLimit    = 1000000 // 1 MB max
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
	commands.AddRootCommands(p, cmdListCommands, cmdFixCommands, cmdEvalCommand, cmdDiagnoseCCTriggers)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleMessageCreate), eventsystem.EventMessageCreate)
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleMessageUpdate), eventsystem.EventMessageUpdate)
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(handleMessageReactions), eventsystem.EventMessageReactionAdd, eventsystem.EventMessageReactionRemove)
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(handleInteractionCreate), eventsystem.EventInteractionCreate)

	pubsub.AddHandler("custom_commands_run_now", handleCustomCommandsRunNow, models.CustomCommand{})
	scheduledevents2.RegisterHandler("cc_next_run", NextRunScheduledEvent{}, handleNextRunScheduledEVent)
	scheduledevents2.RegisterHandler("cc_delayed_run", DelayedRunCCData{}, handleDelayedRunCC)
}

func handleCustomCommandsRunNow(event *pubsub.Event) {
	dataCast := event.Data.(*models.CustomCommand)
	f := logger.WithFields(logrus.Fields{
		"guild_id": dataCast.GuildID,
		"cmd_id":   dataCast.LocalID,
	})

	gs := bot.State.GetGuild(dataCast.GuildID)
	if gs == nil {
		f.Error("failed fetching active guild from state")
		return
	}

	cs := gs.GetChannel(dataCast.ContextChannel)
	if cs == nil {
		f.Error("failed finding channel to run cc in")
		return
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	tmplCtx := templates.NewContext(gs, cs, nil)
	ExecuteCustomCommand(dataCast, tmplCtx)

	dataCast.LastRun = null.TimeFrom(time.Now())
	err := UpdateCommandNextRunTime(dataCast, true, true)
	if err != nil {
		f.WithError(err).Error("failed updating custom command next run time")
	}
}

type DelayedRunCCData struct {
	ChannelID int64  `json:"channel_id"`
	CmdID     int64  `json:"cmd_id"`
	UserData  []byte `json:"data"`

	Message *discordgo.Message
	Member  *dstate.MemberState

	UserKey interface{} `json:"user_key"`

	ExecutedFrom templates.ExecutedFromType `json:"executed_from"`
}

var cmdEvalCommand = &commands.YAGCommand{
	CmdCategory:  commands.CategoryTool,
	Name:         "Evalcc",
	Description:  "executes custom command code (up to 1k characters)",
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "code", Type: dcmd.String},
	},
	SlashCommandEnabled: false,
	DefaultEnabled:      true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		hasCoreWriteRole := false

		writeRoles := common.GetCoreServerConfCached(data.GuildData.GS.ID).AllowedWriteRoles
		for _, r := range data.GuildData.MS.Member.Roles {
			if common.ContainsInt64Slice(writeRoles, r) {
				hasCoreWriteRole = true
				break
			}
		}

		adminOrPerms, err := bot.AdminOrPermMS(data.GuildData.GS.ID, data.GuildData.CS.ID, data.GuildData.MS, discordgo.PermissionManageGuild)
		if err != nil {
			return nil, err
		}

		if !(adminOrPerms || hasCoreWriteRole) {
			return "You need `Manage Server` permissions or control panel write access for this command", nil
		}

		// Disallow calling via exec / execAdmin
		if data.Context().Value(commands.CtxKeyExecutedByCC) == true {
			return "", nil
		}

		channel := data.GuildData.CS
		ctx := templates.NewContext(data.GuildData.GS, channel, data.GuildData.MS)
		ctx.ExecutedFrom = templates.ExecutedFromEvalCC
		ctx.Msg = data.TraditionalTriggerData.Message

		// use stripped message content instead of parsed arg data to avoid dcmd
		// from misinterpreting backslashes and losing spaces in input; see
		// https://github.com/botlabs-gg/yagpdb/pull/1547
		code := common.ParseCodeblock(data.TraditionalTriggerData.MessageStrippedPrefix)

		// Encourage only small code snippets being tested with this command
		maxRunes := 1000
		if utf8.RuneCountInString(code) > maxRunes {
			return "Code is too long for in-place evaluation. Please use the control panel.", nil
		}

		if channel == nil {
			return "Something weird happened... Contact the support server.", nil
		}

		out, err := ctx.Execute(code)

		if err != nil {
			return formatCustomCommandRunErr(code, err), err
		}

		return out, nil
	},
}

type cmdDiagnosisResult int

const (
	cmdOK cmdDiagnosisResult = iota
	cmdExceedsTriggerLimits
	cmdDisabled
	cmdUnmetRestrictions
)

type triggeredCmdDiagnosis struct {
	CC     *models.CustomCommand
	Result cmdDiagnosisResult
}

func (diag triggeredCmdDiagnosis) WriteTo(out *strings.Builder) {
	switch diag.Result {
	case cmdOK:
		out.WriteString(":white_check_mark: ")
	case cmdExceedsTriggerLimits:
		out.WriteString(":warning: ")
	}

	fmt.Fprintf(out, "[**CC #%d**](%s): %s `%s`\n", diag.CC.LocalID, cmdControlPanelLink(diag.CC),
		CommandTriggerType(diag.CC.TriggerType), diag.CC.TextTrigger)
	switch diag.Result {
	case cmdOK:
		out.WriteString("- will execute")
	case cmdExceedsTriggerLimits:
		out.WriteString("- would otherwise execute, but **exceeds limit on commands executed per message**")
	case cmdDisabled:
		out.WriteString("- triggers on input, but **is disabled**")
	case cmdUnmetRestrictions:
		out.WriteString("- triggers on input, but **has unmet role/channel restrictions**")
	}
}

var cmdDiagnoseCCTriggers = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "DiagnoseCCTriggers",
	Aliases:     []string{"debugcctriggers", "diagnosetriggers", "debugtriggers", "dcct"},
	Description: "List all custom commands that would trigger on the input and identify potential issues",
	Arguments: []*dcmd.ArgDef{
		{Name: "input", Type: dcmd.String},
	},
	RequireDiscordPerms: []int64{discordgo.PermissionManageGuild},
	SlashCommandEnabled: false,
	DefaultEnabled:      true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		cmds, err := BotCachedGetCommandsWithMessageTriggers(data.GuildData.GS.ID, data.Context())
		if err != nil {
			return "Failed fetching custom commands", err
		}

		prefix, err := prfx.GetCommandPrefixRedis(data.GuildData.GS.ID)
		if err != nil {
			return "Failed fetching command prefix", err
		}

		// Use the raw input, not dcmd's interpretation of it (which may drop characters.)
		input := data.TraditionalTriggerData.MessageStrippedPrefix

		var triggered []*models.CustomCommand
		for _, cmd := range cmds {
			if matches, _, _ := CheckMatch(prefix, cmd, input); matches {
				triggered = append(triggered, cmd)
			}
		}
		if len(triggered) == 0 {
			return "No commands trigger on the input", nil
		}

		sort.Slice(triggered, func(i, j int) bool { return hasHigherPriority(triggered[i], triggered[j]) })

		limit := CCActionExecLimit(data.GuildData.GS.ID)
		executed, skipped := 0, 0

		diagnoses := make([]triggeredCmdDiagnosis, 0, len(triggered))
		for _, cmd := range triggered {
			switch {
			case cmd.Disabled || (cmd.R.Group != nil && cmd.R.Group.Disabled):
				diagnoses = append(diagnoses, triggeredCmdDiagnosis{cmd, cmdDisabled})
			case !CmdRunsForUser(cmd, data.GuildData.MS):
				diagnoses = append(diagnoses, triggeredCmdDiagnosis{cmd, cmdUnmetRestrictions})
			case !CmdRunsInChannel(cmd, common.ChannelOrThreadParentID(data.GuildData.CS)):
				diagnoses = append(diagnoses, triggeredCmdDiagnosis{cmd, cmdUnmetRestrictions})
			case executed >= limit:
				skipped++
				diagnoses = append(diagnoses, triggeredCmdDiagnosis{cmd, cmdExceedsTriggerLimits})
			default:
				executed++
				diagnoses = append(diagnoses, triggeredCmdDiagnosis{cmd, cmdOK})
			}
		}

		var out strings.Builder
		if skipped > 0 {
			fmt.Fprintf(&out, `> ### Potential issue detected
> Not all custom commands triggering on the input will actually be executed.
> Note that at most %d custom commands can be executed by a single message.`, limit)
			out.WriteByte('\n')
		}
		out.WriteString("## Commands triggering on input\n")
		for _, diagnosis := range diagnoses {
			diagnosis.WriteTo(&out)
			out.WriteByte('\n')
		}
		msg := &discordgo.MessageSend{
			Flags:   discordgo.MessageFlagsSuppressEmbeds,
			Content: out.String(),
		}
		return msg, nil
	},
}

var cmdListCommands = &commands.YAGCommand{
	CmdCategory:    commands.CategoryTool,
	Name:           "CustomCommands",
	Aliases:        []string{"cc"},
	Description:    "Shows a custom command specified by id, trigger, or name, or lists them all",
	ArgumentCombos: [][]int{{0}, {1}, {}},
	Arguments: []*dcmd.ArgDef{
		{Name: "ID", Type: dcmd.Int},
		{Name: "Name-Or-Trigger", Type: dcmd.String},
	},
	SlashCommandEnabled: true,
	DefaultEnabled:      false,
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "file", Help: "Send responses in file"},
		{Name: "color", Help: "Use syntax highlighting (Go)"},
		{Name: "raw", Help: "Force raw output"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		ccs, err := models.CustomCommands(qm.Where("guild_id = ?", data.GuildData.GS.ID), qm.OrderBy("local_id")).AllG(data.Context())
		if err != nil {
			return "Failed retrieving custom commands", err
		}

		groups, err := models.CustomCommandGroups(qm.Where("guild_id=?", data.GuildData.GS.ID)).AllG(data.Context())
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
				return "No command by that id, trigger or name found, here is a list of them all:\n" + list, nil
			} else {
				return "No id or trigger provided, here is a list of all server commands:\n" + list, nil
			}
		}

		if len(foundCCS) > 1 {
			return "More than 1 matched command\n" + StringCommands(foundCCS, groupMap), nil
		}

		cc := foundCCS[0]

		highlight := "txt"
		if data.Switches["color"].Value != nil {
			highlight = "go"
		}

		var ccFile *discordgo.File
		msg := &discordgo.MessageSend{Flags: discordgo.MessageFlagsSuppressEmbeds}

		responses := fmt.Sprintf("```\n%s\n```", strings.Join(cc.Responses, "```\n```"))
		if data.Switches["file"].Value != nil || len(responses) > 1500 && data.Switches["raw"].Value == nil {
			var buf bytes.Buffer
			buf.WriteString(strings.Join(cc.Responses, "\nAdditional response:\n"))

			ccFile = &discordgo.File{
				Name:   fmt.Sprintf("%s_CC_%d.%s", data.GuildData.GS.Name, cc.LocalID, highlight),
				Reader: &buf,
			}
		}

		ccIDMaybeWithLink := strconv.FormatInt(cc.LocalID, 10)

		// Add public link to the CC if it is public
		if cc.Public && cc.PublicID != "" {
			ccIDMaybeWithLink = fmt.Sprintf("[%d](%s/cc/%s)", cc.LocalID, web.BaseURL(), cc.PublicID)
		}

		// Add link to the cc on dashboard if member has read access
		var member *discordgo.Member
		if data.TriggerType == dcmd.TriggerTypeSlashCommands {
			member = data.SlashCommandTriggerData.Interaction.Member
		} else {
			member = data.TraditionalTriggerData.Message.Member
		}
		roles := member.Roles
		perms, _ := data.GuildData.GS.GetMemberPermissions(data.ChannelID, data.Author.ID, roles)

		gWithConnected := &common.GuildWithConnected{
			UserGuild: &discordgo.UserGuild{
				ID:          data.GuildData.GS.ID,
				Owner:       data.Author.ID == data.GuildData.GS.OwnerID,
				Permissions: perms,
			},
			Connected: true,
		}

		if hasRead, _ := web.GetUserAccessLevel(data.Author.ID, gWithConnected, common.GetCoreServerConfCached(data.GuildData.GS.ID), web.StaticRoleProvider(roles)); hasRead {
			ccIDMaybeWithLink = fmt.Sprintf("[%d](%s)", cc.LocalID, cmdControlPanelLink(cc))
		}

		// Every message content-based custom command trigger has a numerical value less than 5
		if cc.TriggerType < 5 || cc.TriggerType == int(CommandTriggerComponent) || cc.TriggerType == int(CommandTriggerModal) {
			var header string
			if cc.TextTrigger == "" {
				cc.TextTrigger = `​`
			}
			if cc.Name.Valid {
				header = fmt.Sprintf("#%s - Trigger: `%s` - Type: `%s` - Name: `%s` - Case sensitive trigger: `%t` - Group: `%s` - Disabled: `%t` - onEdit: `%t` Public: `%t`",
					ccIDMaybeWithLink, cc.TextTrigger, CommandTriggerType(cc.TriggerType), cc.Name.String, cc.TextTriggerCaseSensitive, groupMap[cc.GroupID.Int64], cc.Disabled, cc.TriggerOnEdit, cc.Public)
			} else {
				header = fmt.Sprintf("#%s - Trigger: `%s` - Type: `%s` - Case sensitive trigger: `%t` - Group: `%s` - Disabled: `%t` - onEdit: `%t` Public: `%t`",
					ccIDMaybeWithLink, cc.TextTrigger, CommandTriggerType(cc.TriggerType), cc.TextTriggerCaseSensitive, groupMap[cc.GroupID.Int64], cc.Disabled, cc.TriggerOnEdit, cc.Public)
			}

			if ccFile != nil {
				msg.Content = header
				msg.Files = []*discordgo.File{ccFile}
				return msg, nil
			}

			msg.Content = fmt.Sprintf("%s\n```%s\n%s\n```", header, highlight, strings.Join(cc.Responses, "```\n```"))
			return msg, nil
		}

		if ccFile != nil {
			var header string
			if cc.Name.Valid {
				header = fmt.Sprintf("#%s - Type: `%s` - Name: `%s` - Group: `%s` - Disabled: `%t` Public: `%t`", ccIDMaybeWithLink, CommandTriggerType(cc.TriggerType), cc.Name.String, groupMap[cc.GroupID.Int64], cc.Disabled, cc.Public)
			} else {
				header = fmt.Sprintf("#%s - Type: `%s` - Group: `%s` - Disabled: `%t` Public: `%t`", ccIDMaybeWithLink, CommandTriggerType(cc.TriggerType), groupMap[cc.GroupID.Int64], cc.Disabled, cc.Public)
			}

			msg.Content = header
			msg.Files = []*discordgo.File{ccFile}

			return msg, nil

		}

		if cc.Name.Valid {
			msg.Content = fmt.Sprintf("#%s - Type: `%s` - Name: `%s` - Group: `%s` - Disabled: `%t` Public: `%t`\n```%s\n%s\n```",
				ccIDMaybeWithLink, CommandTriggerType(cc.TriggerType), cc.Name.String, groupMap[cc.GroupID.Int64], cc.Disabled, cc.Public,
				highlight, strings.Join(cc.Responses, "```\n```"))
		} else {
			msg.Content = fmt.Sprintf("#%s - Type: `%s` - Group: `%s` - Disabled: `%t` Public: `%t`\n```%s\n%s\n```",
				ccIDMaybeWithLink, CommandTriggerType(cc.TriggerType), groupMap[cc.GroupID.Int64], cc.Disabled, cc.Public,
				highlight, strings.Join(cc.Responses, "```\n```"))
		}
		return msg, nil
	},
}

func cmdControlPanelLink(cmd *models.CustomCommand) string {
	return fmt.Sprintf("%s/customcommands/commands/%d/", web.ManageServerURL(cmd.GuildID), cmd.LocalID)
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
		// Find by trigger/name
		nameOrTrigger := data.Args[1].Str()
		for _, v := range ccs {
			if strings.EqualFold(nameOrTrigger, v.TextTrigger) || (v.Name.Valid && strings.EqualFold(nameOrTrigger, v.Name.String)) {
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
		switch cc.TriggerType {
		case int(CommandTriggerReaction), int(CommandTriggerInterval), int(CommandTriggerNone):
			if cc.Name.Valid {
				out += fmt.Sprintf("`#%3d:` - Type: `%s` - Name: `%s` - Group: `%s` - Disabled: `%t` - Public: `%t`\n", cc.LocalID, CommandTriggerType(cc.TriggerType).String(), cc.Name.String, gMap[cc.GroupID.Int64], cc.Disabled, cc.Public)
			} else {
				out += fmt.Sprintf("`#%3d:` - Type: `%s` - Group: `%s` - Disabled: `%t` - Public: `%t`\n", cc.LocalID, CommandTriggerType(cc.TriggerType).String(), gMap[cc.GroupID.Int64], cc.Disabled, cc.Public)
			}
		default:
			if cc.TextTrigger == "" {
				cc.TextTrigger = `​`
			}
			if cc.Name.Valid {
				out += fmt.Sprintf("`#%3d:` - Trigger: `%s` - Type: `%s`  - Name: `%s` - Group: `%s` - Disabled: `%t` - onEdit: `%t` - Public: `%t`\n", cc.LocalID, cc.TextTrigger, CommandTriggerType(cc.TriggerType).String(), cc.Name.String, gMap[cc.GroupID.Int64], cc.Disabled, cc.TriggerOnEdit, cc.Public)
			} else {
				out += fmt.Sprintf("`#%3d:` - Trigger: `%s` - Type: `%s` - Group: `%s` - Disabled: `%t` - onEdit: `%t` - Public: `%t`\n", cc.LocalID, cc.TextTrigger, CommandTriggerType(cc.TriggerType).String(), gMap[cc.GroupID.Int64], cc.Disabled, cc.TriggerOnEdit, cc.Public)
			}
		}
	}

	return out
}

func handleDelayedRunCC(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*DelayedRunCCData)
	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", evt.GuildID, dataCast.CmdID), qm.Load("Group")).OneG(context.Background())
	if err != nil {
		return false, errors.WrapIf(err, "find_command")
	}

	if cmd.R.Group != nil && cmd.R.Group.Disabled {
		return false, errors.New("custom command group is disabled")
	}

	if cmd.Disabled {
		return false, errors.New("custom command is disabled")
	}

	if !DelayedCCRunLimit.AllowN(DelayedRunLimitKey{GuildID: evt.GuildID, ChannelID: dataCast.ChannelID}, time.Now(), 1) {
		logger.WithField("guild", cmd.GuildID).Warn("went above delayed cc run ratelimit")
		return false, nil
	}

	gs := bot.State.GetGuild(evt.GuildID)
	if gs == nil {
		// in case the bot left in the meantime
		if onGuild, err := common.BotIsOnGuild(evt.GuildID); !onGuild && err == nil {
			return false, nil
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}

		return true, nil
	}

	cs := gs.GetChannelOrThread(dataCast.ChannelID)
	if cs == nil {
		// don't reschedule if channel is deleted, make sure its actually not there, and not just a discord downtime
		if !gs.Available {
			return true, nil
		}

		return false, nil
	}

	// attempt to get up to date member information
	if dataCast.Member != nil {
		updatedMS, _ := bot.GetMember(gs.ID, dataCast.Member.User.ID)
		if updatedMS != nil {
			dataCast.Member = updatedMS
		}
	}

	tmplCtx := templates.NewContext(gs, cs, dataCast.Member)
	if dataCast.Message != nil {
		tmplCtx.Msg = dataCast.Message
		tmplCtx.Data["Message"] = dataCast.Message
	}

	tmplCtx.ExecutedFrom = dataCast.ExecutedFrom

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
	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", evt.GuildID, (data.(*NextRunScheduledEvent)).CmdID), qm.Load("Group")).OneG(context.Background())
	if err != nil {
		return false, errors.WrapIf(err, "find_command")
	}

	if cmd.R.Group != nil && cmd.R.Group.Disabled {
		return false, errors.New("custom command group is disabled")
	}

	if cmd.Disabled {
		return false, errors.New("custom command is disabled")
	}

	if time.Until(cmd.NextRun.Time) > time.Second*5 {
		return false, nil // old scheduled event that wasn't removed, /shrug

	}

	gs := bot.State.GetGuild(evt.GuildID)
	if gs == nil {
		if onGuild, err := common.BotIsOnGuild(evt.GuildID); !onGuild && err == nil {
			return false, nil
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}

		return true, nil
	}

	cs := gs.GetChannel(cmd.ContextChannel)
	if cs == nil {
		// don't reschedule if channel is deleted, make sure its actually not there, and not just a discord downtime
		if !gs.Available {
			return true, nil
		}

		return false, nil
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	tmplCtx := templates.NewContext(gs, cs, nil)
	ExecuteCustomCommand(cmd, tmplCtx)

	// schedule next runs
	cmd.LastRun = cmd.NextRun
	err = UpdateCommandNextRunTime(cmd, true, false)
	if err != nil {
		logger.WithError(err).Error("failed updating custom command next run time")
	}

	return false, nil
}

func shouldIgnoreChannel(msg *discordgo.Message, gs *dstate.GuildSet, cState *dstate.ChannelState) bool {
	if msg.GuildID == 0 {
		return true
	}

	if cState == nil {
		logger.Warn("Channel not found in state")
		return true
	}

	if !bot.IsUserMessage(msg) {
		return true
	}

	if msg.Author.Bot {
		return true
	}

	if hasPerms, _ := bot.BotHasPermissionGS(gs, cState.ID, discordgo.PermissionSendMessages); !hasPerms {
		return true
	}

	// Passed all checks, custom commands should not ignore this channel
	return false
}

// Limit the number of custom commands that can be executed on a single action (messages, reactions,
// component uses, and so on).

const (
	CCActionExecLimitNormal  = 3
	CCActionExecLimitPremium = 5
)

func CCActionExecLimit(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return CCActionExecLimitPremium
	}
	return CCActionExecLimitNormal
}

func (p *Plugin) OnRemovedPremiumGuild(GuildID int64) error {
	_, err := models.CustomCommands(qm.Where("guild_id = ? AND length(regexp_replace(array_to_string(responses, ''), E'\\r', '', 'g')) > ?", GuildID, MaxCCResponsesLength)).UpdateAllG(context.Background(), models.M{"disabled": true})
	if err != nil {
		return errors.WrapIf(err, "Failed disabling long custom commands on premium removal")
	}

	commands, err := models.CustomCommands(qm.Where("guild_id = ? AND disabled = false", GuildID), qm.OrderBy("local_id ASC"), qm.Offset(MaxCommands)).AllG(context.Background())
	if err != nil {
		return errors.WrapIf(err, "failed getting custom commands")
	}

	if len(commands) > 0 {
		_, err = commands.UpdateAllG(context.Background(), models.M{"disabled": true})
		if err != nil {
			return errors.WrapIf(err, "failed disabling custom commands on premium removal")
		}
	}
	_, err = models.CustomCommands(qm.Where("guild_id = ?", GuildID)).UpdateAllG(context.Background(), models.M{"trigger_on_edit": false})
	if err != nil {
		return errors.WrapIf(err, "Failed disabling trigger on edits on premium removal")
	}

	return nil
}

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
		cID, ok := strings.CutPrefix(cID, "templates-")
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

		for _, matched := range triggeredCmds {
			err = ExecuteCustomCommandFromComponent(matched.CC, evt.GS, cState, matched.Args, matched.Stripped, &interaction)
			if err != nil {
				logger.WithField("guild", cState.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
			}
		}
	case discordgo.InteractionModalSubmit:
		cID := interaction.ModalSubmitData().CustomID

		// continue only if this modal was created by a cc
		cID, ok := strings.CutPrefix(cID, "templates-")
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

		for _, matched := range triggeredCmds {
			err = ExecuteCustomCommandFromModal(matched.CC, evt.GS, cState, matched.Args, matched.Stripped, &interaction)
			if err != nil {
				logger.WithField("guild", cState.GuildID).WithField("cc_id", matched.CC.LocalID).WithError(err).Error("Error executing custom command")
			}
		}
	}
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

type TriggeredCC struct {
	CC       *models.CustomCommand
	Stripped string
	Args     []string
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

func ExecuteCustomCommandFromMessage(gs *dstate.GuildSet, cmd *models.CustomCommand, member *dstate.MemberState, cs *dstate.ChannelState, cmdArgs []string, stripped string, m *discordgo.Message, isEdit bool) error {
	tmplCtx := templates.NewContext(gs, cs, member)
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
	tmplCtx.Data["IsMessageEdit"] = isEdit
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
	tmplCtx.Data["CCTrigger"] = cmd.TextTrigger

	csCop := tmplCtx.CurrentFrame.CS
	f := logger.WithFields(logrus.Fields{
		"trigger":      cmd.TextTrigger,
		"trigger_type": CommandTriggerType(cmd.TriggerType).String(),
		"guild":        csCop.GuildID,
		"channel_name": csCop.Name,
	})

	// do not allow concurrent executions of the same custom command, to prevent most common kinds of abuse
	lockKey := CCExecKey{
		GuildID: cmd.GuildID,
		CCID:    cmd.LocalID,
	}
	lockHandle := CCExecLock.Lock(lockKey, time.Minute, time.Minute*10)
	if lockHandle == -1 {
		f.Warn("Exceeded max lock attempts for cc")
		errChannel := tmplCtx.CurrentFrame.CS.ID
		if cmd.RedirectErrorsChannel != 0 {
			errChannel = cmd.RedirectErrorsChannel
		}

		if cmd.ShowErrors {
			common.BotSession.ChannelMessageSend(errChannel, fmt.Sprintf("Gave up trying to execute custom command #%d after 1 minute because there is already one or more instances of it being executed.", cmd.LocalID))
		}
		updatePostCommandRan(cmd, errors.New("Gave up trying to execute, already an existing instance executing"))
		return nil
	}

	defer CCExecLock.Unlock(lockKey, lockHandle)

	go analytics.RecordActiveUnit(cmd.GuildID, &Plugin{}, "executed_cc")

	// pick a response and execute it
	f.Debug("Custom command triggered")

	chanMsg := cmd.Responses[rand.Intn(len(cmd.Responses))]
	out, err := tmplCtx.Execute(chanMsg)

	// trim whitespace for accurate character count
	out = strings.TrimSpace(out)

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command (#" + discordgo.StrID(cmd.LocalID) + ") response was longer than 2k (contact an admin on the server...)"
	}

	go updatePostCommandRan(cmd, err)

	// deal with the results
	if err != nil {
		logger.WithField("guild", tmplCtx.GS.ID).WithError(err).Error("Error executing custom command")

		errChannel := tmplCtx.CurrentFrame.CS.ID
		if cmd.RedirectErrorsChannel != 0 {
			errChannel = cmd.RedirectErrorsChannel
		}

		if cmd.ShowErrors {
			out += "\nAn error caused the execution of the custom command template to stop:\n"
			out += formatCustomCommandRunErr(chanMsg, err)

			common.BotSession.ChannelMessageSend(errChannel, out)
			return nil
		}
	}

	_, err = tmplCtx.SendResponse(out)
	if err != nil {
		return errors.WithStackIf(err)
	}
	return nil
}

func ExecuteCustomCommandFromComponent(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, cmdArgs []string, stripped string, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, ms)
	tmplCtx.CurrentFrame.Interaction = interaction

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = interaction.MessageComponentData()
	cid := strings.TrimPrefix(interaction.MessageComponentData().CustomID, "templates-")
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

func ExecuteCustomCommandFromModal(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, cmdArgs []string, stripped string, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, ms)
	tmplCtx.CurrentFrame.Interaction = interaction

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = interaction.ModalSubmitData()
	cid := strings.TrimPrefix(interaction.ModalSubmitData().CustomID, "templates-")
	tmplCtx.Data["CustomID"] = cid
	tmplCtx.Data["Cmd"] = cmdArgs[0]
	if len(cmdArgs) > 1 {
		tmplCtx.Data["CmdArgs"] = cmdArgs[1:]
	} else {
		tmplCtx.Data["CmdArgs"] = []string{}
	}
	tmplCtx.Data["StrippedID"] = stripped
	tmplCtx.Data["StrippedMsg"] = stripped
	tmplCtx.Data["IsModal"] = true
	cmdValues := []string{}
	for i := 0; i < len(interaction.ModalSubmitData().Components); i++ {
		row := interaction.ModalSubmitData().Components[i].(*discordgo.ActionsRow)
		field := row.Components[0].(*discordgo.TextInput)
		cmdValues = append(cmdValues, field.Value)
	}
	tmplCtx.Data["Values"] = cmdValues

	msg := interaction.Message
	msg.Member = ms.DgoMember()
	msg.Author = msg.Member.User
	tmplCtx.Msg = msg

	tmplCtx.Data["Message"] = msg

	return ExecuteCustomCommand(cc, tmplCtx)
}

func formatCustomCommandRunErr(src string, err error) string {
	// check if we can retrieve the original ExecError
	cause := errors.Cause(err)
	if eerr, ok := cause.(template.ExecError); ok {
		data := parseExecError(eerr)
		// couldn't parse error, fall back to the original error message
		if data == nil {
			return "`" + err.Error() + "`"
		}

		out := fmt.Sprintf("`Failed executing CC #%d, line %d, row %d: %s`", data.CCID, data.Line, data.Row, data.Msg)
		lines := strings.Split(src, "\n")
		if len(lines) < int(data.Line) {
			return out
		}

		out += "\n```"
		out += getSurroundingLines(lines, int(data.Line-1)) // data.Line is 1-based, convert to 0-based.
		out += "\n```"
		return out
	}

	// otherwise, fall back to the normal error message
	return "`" + err.Error() + "`"
}

// getSurroundingLines returns a string representing the lines close to a given
// line number with common leading whitespace removed. Each line is formatted like
// `<line number>    <line content>`.
func getSurroundingLines(lines []string, lineIndex int) string {
	var lineNums []int
	var res []string

	commonLeadingSpaces := -1
	addLine := func(n int) {
		line := lines[n]

		leadingSpaceCount := 0
		var cleaned strings.Builder
		var i int
	Loop:
		for i = 0; i < len(line); i++ {
			switch line[i] {
			case '\t':
				cleaned.WriteString("    ") // tabs -> 4 spaces
				leadingSpaceCount += 4
			case ' ':
				cleaned.WriteByte(' ')
				leadingSpaceCount++
			default:
				break Loop
			}
		}

		if i != len(line) {
			cleaned.WriteString(line[i:])
		}

		if commonLeadingSpaces == -1 || leadingSpaceCount < commonLeadingSpaces {
			commonLeadingSpaces = leadingSpaceCount
		}

		res = append(res, cleaned.String())
		lineNums = append(lineNums, n+1) // line numbers shown to the user are 1-based
	}

	// add previous line if possible
	if lineIndex > 0 && len(lines) > 1 {
		addLine(lineIndex - 1)
	}

	addLine(lineIndex)

	// add next line if possible
	if lineIndex != len(lines)-1 {
		addLine(lineIndex + 1)
	}

	var out strings.Builder
	for i, line := range res {
		if i > 0 {
			out.WriteByte('\n')
		}

		// remove common leading whitespace
		line = line[commonLeadingSpaces:]
		if len(line) > 35 {
			line = limitString(line, 30) + "..."
		}
		// replace all ` with ` + a ZWS to make sure that all the code will stay formatted nicely in the codeblock
		line = strings.ReplaceAll(line, "`", "`\u200b")

		out.WriteString(strconv.FormatInt(int64(lineNums[i]), 10))
		out.WriteString("    ")
		out.WriteString(line)
	}

	return out.String()
}

type execErrorData struct {
	CCID, Line, Row int64
	Msg             string
}

var execErrorInfoRe = regexp.MustCompile(`\Atemplate: CC #(\d+):(\d+):(\d+): ([\S\s]+)`)

// parseExecError uses regex to extract the individual parts out of an ExecError.
// It returns nil if an error occurred during parsing.
func parseExecError(err template.ExecError) *execErrorData {
	parts := execErrorInfoRe.FindStringSubmatch(err.Error())
	if parts == nil {
		return nil
	}

	ccid, perr := strconv.ParseInt(parts[1], 10, 64)
	if perr != nil {
		return nil
	}

	line, perr := strconv.ParseInt(parts[2], 10, 64)
	if perr != nil {
		return nil
	}

	row, perr := strconv.ParseInt(parts[3], 10, 64)
	if perr != nil {
		return nil
	}

	return &execErrorData{ccid, line, row, parts[4]}
}

func onExecPanic(cmd *models.CustomCommand, err error, tmplCtx *templates.Context, logStack bool) {
	l := logger.WithField("guild", tmplCtx.GS.ID).WithError(err)
	if logStack {
		stack := string(debug.Stack())
		l = l.WithField("stack", stack)
	}

	l.Error("Error executing custom command")

	errChannel := tmplCtx.CurrentFrame.CS.ID
	if cmd.RedirectErrorsChannel != 0 {
		errChannel = cmd.RedirectErrorsChannel
	}

	if cmd.ShowErrors {
		out := "\nAn error caused the execution of the custom command template to stop:\n"
		out += "`" + err.Error() + "`"

		common.BotSession.ChannelMessageSend(errChannel, out)
	}

	updatePostCommandRan(cmd, err)
}

func updatePostCommandRan(cmd *models.CustomCommand, runErr error) {
	const qNoErr = "UPDATE custom_commands SET run_count = run_count + 1, last_run=now() WHERE guild_id=$1 AND local_id=$2"
	const qErr = "UPDATE custom_commands SET run_count = run_count + 1, last_run=now(), last_error=$3, last_error_time=now() WHERE guild_id=$1 AND local_id=$2"

	var err error
	if runErr == nil {
		_, err = common.PQ.Exec(qNoErr, cmd.GuildID, cmd.LocalID)
	} else {
		_, err = common.PQ.Exec(qErr, cmd.GuildID, cmd.LocalID, runErr.Error())
	}

	if err != nil {
		logger.WithError(err).WithField("guild", cmd.GuildID).Error("failed running post command executed query")
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

var cmdFixCommands = &commands.YAGCommand{
	CmdCategory:          commands.CategoryTool,
	Name:                 "fixscheduledccs",
	Description:          "Corrects the next run time of interval CCs globally, fixes issues arising from missed executions due to downtime. Bot Owner Only",
	HideFromCommandsPage: true,
	HideFromHelp:         true,
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		ccs, err := models.CustomCommands(qm.Where("trigger_type = 5"), qm.Where("now() - INTERVAL '1 hour' > next_run"), qm.Where("disabled = false")).AllG(context.Background())
		if err != nil {
			return nil, err
		}

		for _, v := range ccs {
			err = UpdateCommandNextRunTime(v, false, false)
			if err != nil {
				return nil, err
			}
		}

		return fmt.Sprintf("Doneso! fixed %d commands!", len(ccs)), nil
	}),
}
