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
	"github.com/botlabs-gg/yagpdb/v2/common/keylock"
	"github.com/botlabs-gg/yagpdb/v2/common/multiratelimit"
	prfx "github.com/botlabs-gg/yagpdb/v2/common/prefix"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
	"github.com/sirupsen/logrus"
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
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(handleGuildAuditLogEntryCreate), eventsystem.EventGuildAuditLogEntryCreate)

	pubsub.AddHandler("custom_commands_run_now", handleCustomCommandsRunNow, models.CustomCommand{})
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

	ExecutedFrom templates.ExecutedFromType `json:"executed_from"`
}

var cmdEvalCommand = &commands.YAGCommand{
	CmdCategory:  commands.CategoryTool,
	Name:         "Evalcc",
	Description:  "executes custom command code.",
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
		ctx.Data["Message"] = ctx.Msg

		// use stripped message content instead of parsed arg data to avoid dcmd
		// from misinterpreting backslashes and losing spaces in input; see
		// https://github.com/botlabs-gg/yagpdb/pull/1547
		code := common.ParseCodeblock(data.TraditionalTriggerData.MessageStrippedPrefix)

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

func (diag triggeredCmdDiagnosis) WriteTo(out *strings.Builder, includeLink bool) {
	switch diag.Result {
	case cmdOK:
		out.WriteString("✅ ")
	case cmdExceedsTriggerLimits:
		out.WriteString("⚠️ ")
	}

	if includeLink {
		fmt.Fprintf(out, "[**CC #%d**](%s): %s `%s`\n", diag.CC.LocalID, cmdControlPanelLink(diag.CC),
			CommandTriggerType(diag.CC.TriggerType), diag.CC.TextTrigger)
	} else {
		fmt.Fprintf(out, "CC %d: %s `%s`\n", diag.CC.LocalID, CommandTriggerType(diag.CC.TriggerType), diag.CC.TextTrigger)
	}
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
	SlashCommandEnabled: true,
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

		const inlineThreshold = 5 // If there's more than this many diagnostics, output to a file instead.
		if len(diagnoses) <= inlineThreshold {
			out.WriteString("## Commands triggering on input\n")
			for _, diagnosis := range diagnoses {
				diagnosis.WriteTo(&out, true)
				out.WriteByte('\n')
			}
			return &discordgo.MessageSend{
				Flags:   discordgo.MessageFlagsSuppressEmbeds,
				Content: out.String(),
			}, nil
		}

		var fileOut strings.Builder
		for _, diag := range diagnoses {
			diag.WriteTo(&fileOut, false)
			fileOut.WriteString("\n\n")
		}
		return &discordgo.MessageSend{
			Content: out.String(),
			File:    &discordgo.File{Name: "output.txt", Reader: strings.NewReader(fileOut.String())},
		}, nil
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

	commands, err := models.CustomCommands(qm.Where("guild_id = ? AND disabled = false AND trigger_type = ?", GuildID, CommandTriggerRole), qm.OrderBy("local_id ASC	"), qm.Offset(MaxRoleTriggerCommands)).AllG(context.Background())
	if err != nil {
		return errors.WrapIf(err, "failed fetching role trigger custom commands on premium removal")
	}
	if len(commands) > 0 {
		_, err = commands.UpdateAllG(context.Background(), models.M{"disabled": true})
		if err != nil {
			return errors.WrapIf(err, "failed disabling role trigger custom commands on premium removal")
		}
	}

	commands, err = models.CustomCommands(qm.Where("guild_id = ? AND disabled = false", GuildID), qm.OrderBy("local_id ASC"), qm.Offset(MaxCommands)).AllG(context.Background())
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

type TriggeredCC struct {
	CC       *models.CustomCommand
	Stripped string
	Args     []string
}

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
		} else if cmd.R.Group != nil && cmd.R.Group.RedirectErrorsChannel != 0 {
			errChannel = cmd.R.Group.RedirectErrorsChannel
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
		} else if cmd.R.Group != nil && cmd.R.Group.RedirectErrorsChannel != 0 {
			errChannel = cmd.R.Group.RedirectErrorsChannel
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
	} else if cmd.R.Group != nil && cmd.R.Group.RedirectErrorsChannel != 0 {
		errChannel = cmd.R.Group.RedirectErrorsChannel
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
