package customcommands

import (
	"context"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/customcommands/models"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"math/rand"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(cmdListCommands)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(HandleMessageCreate, eventsystem.EventMessageCreate)

	// add the pubsub handler for cache eviction
	pubsub.AddHandler("custom_commands_clear_cache", func(event *pubsub.Event) {
		gs := bot.State.Guild(true, event.TargetGuildInt)
		if gs == nil {
			return
		}

		gs.UserCacheDel(true, CacheKeyCommands)
	}, nil)
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
		ccs, err := models.CustomCommands(qm.Where("guild_id = ?", data.GS.ID)).AllG(data.Context())
		if err != nil {
			return "Failed retrieving custom commands", err
		}

		foundCCS, provided := FindCommands(ccs, data)
		if len(foundCCS) < 1 {
			list := StringCommands(ccs)
			if provided {
				return "No command by that name or id found, here is a list of them all:\n" + list, nil
			} else {
				return "No id or trigger provided, here is a list of all server commands:\n" + list, nil
			}
		}

		if len(foundCCS) > 1 {
			return "More than 1 matched command\n" + StringCommands(foundCCS), nil
		}

		cc := foundCCS[0]

		return fmt.Sprintf("#%d - %s: `%s` - Case sensitive trigger: `%t` \n```\n%s\n```",
			cc.LocalID, CommandTriggerType(cc.TriggerType), cc.TextTrigger, cc.TextTriggerCaseSensitive, strings.Join(cc.Responses, "```\n```")), nil
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

func StringCommands(ccs []*models.CustomCommand) string {
	out := ""
	for _, cc := range ccs {
		out += fmt.Sprintf("`#%3d:` `%s`: %s\n", cc.LocalID, cc.TextTrigger, CommandTriggerType(cc.TriggerType).String())
	}

	return out
}

func shouldIgnoreChannel(evt *discordgo.MessageCreate, cState *dstate.ChannelState) bool {
	if evt.GuildID == 0 {
		return true
	}

	if cState == nil {
		log.Warn("Channel not found in state")
		return true
	}

	botID := common.BotUser.ID

	if evt.Author == nil || botID == evt.Author.ID || evt.Author.Bot || cState.IsPrivate() || evt.WebhookID != 0 {
		return true
	}

	if !bot.BotProbablyHasPermissionGS(true, cState.Guild, cState.ID, discordgo.PermissionSendMessages) {
		return true
	}

	// Passed all checks, custom commands should not ignore this channel
	return false
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	mc := evt.MessageCreate()
	cs := bot.State.Channel(true, mc.ChannelID)

	if shouldIgnoreChannel(mc, cs) {
		return
	}

	cmds, err := BotCachedGetCommandsWithMessageTriggers(cs.Guild, evt.Context())
	if err != nil {
		log.WithError(err).WithField("guild", cs.Guild.ID).Error("Failed retrieving comamnds")
		return
	}

	if len(cmds) < 1 {
		return
	}

	prefix, err := commands.GetCommandPrefix(cs.Guild.ID)
	if err != nil {
		log.WithError(err).Error("Failed getting prefix")
		return
	}

	member, err := bot.GetMember(cs.Guild.ID, mc.Author.ID)
	if err != nil {
		return
	}

	var matched *models.CustomCommand
	var stripped string
	var args []string
	for _, cmd := range cmds {
		if !CmdRunsInChannel(cmd, mc.ChannelID) || !CmdRunsForUser(cmd, member) {
			continue
		}

		if m, s, a := CheckMatch(prefix, cmd, mc.Content); m {
			matched = cmd
			stripped = s
			args = a
			break
		}
	}

	if matched == nil || len(matched.Responses) == 0 {
		return
	}

	if common.Statsd != nil {
		go common.Statsd.Incr("yagpdb.cc.executed", nil, 1)
	}

	channel := cs.Copy(true, true)
	log.WithFields(log.Fields{
		"trigger":      matched.TextTrigger,
		"trigger_type": matched.TriggerType,
		"guild":        channel.Guild.ID,
		"channel_name": channel.Name,
	}).Info("Custom command triggered")

	out, tmplCtx, err := ExecuteCustomCommand(matched, args, stripped, bot.ContextSession(evt.Context()), mc)
	if err != nil {
		log.WithField("guild", channel.Guild.ID).WithError(err).Error("Error executing custom command")
		out += "\nAn error caused the execution of the custom command template to stop:\n"
		out += common.EscapeSpecialMentions(err.Error())
	}

	for _, v := range tmplCtx.EmebdsToSend {
		common.BotSession.ChannelMessageSendEmbed(mc.ChannelID, v)
	}

	if tmplCtx.DelTrigger {
		templates.MaybeScheduledDeleteMessage(mc.GuildID, mc.ChannelID, mc.ID, tmplCtx.DelTriggerDelay)
	}

	if strings.TrimSpace(out) != "" && (!tmplCtx.DelResponse || tmplCtx.DelResponseDelay > 0) {
		m, err := common.BotSession.ChannelMessageSend(mc.ChannelID, out)
		if err != nil {
			log.WithError(err).Error("Failed sending message")
		} else {
			if tmplCtx.DelResponse {
				templates.MaybeScheduledDeleteMessage(mc.GuildID, mc.ChannelID, m.ID, tmplCtx.DelResponseDelay)
			}

			if len(tmplCtx.AddResponseReactionNames) > 0 {
				go func() {
					for _, v := range tmplCtx.AddResponseReactionNames {
						common.BotSession.MessageReactionAdd(m.ChannelID, m.ID, v)
					}
				}()
			}
		}
	}
}

func ExecuteCustomCommand(cmd *models.CustomCommand, cmdArgs []string, stripped string, s *discordgo.Session, m *discordgo.MessageCreate) (resp string, tmplCtx *templates.Context, err error) {

	cs := bot.State.Channel(true, m.ChannelID)
	member, err := bot.GetMember(cs.Guild.ID, m.Author.ID)
	if err != nil {
		err = err
		return
	}

	tmplCtx = templates.NewContext(cs.Guild, cs, member)
	tmplCtx.Msg = m.Message

	args := dcmd.SplitArgs(m.Content)
	argsStr := make([]string, len(args))
	for k, v := range args {
		argsStr[k] = v.Str
	}

	// TODO: Potentially retire undocumented StrippedMsg.
	tmplCtx.Data["Args"] = argsStr
	tmplCtx.Data["StrippedMsg"] = stripped
	tmplCtx.Data["Cmd"] = cmdArgs[0]
	if len(cmdArgs) > 1 {
		tmplCtx.Data["CmdArgs"] = cmdArgs[1:]
	} else {
		tmplCtx.Data["CmdArgs"] = []string{}
	}
	tmplCtx.Data["Message"] = m.Message

	chanMsg := cmd.Responses[rand.Intn(len(cmd.Responses))]
	out, err := tmplCtx.Execute(chanMsg)

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command response was longer than 2k (contact an admin on the server...)"
	}
	resp = out

	return
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
		// ^(<@!?bot_id> ?|server_cmd_prefix)trigger($|[[:space:]])
		cmdMatch += "^(<@!?" + discordgo.StrID(common.BotUser.ID) + "> ?|" + regexp.QuoteMeta(globalPrefix) + ")" + regexp.QuoteMeta(trigger) + "($|[[:space:]])"
	case CommandTriggerStartsWith:
		cmdMatch += "^" + regexp.QuoteMeta(trigger)
	case CommandTriggerContains:
		cmdMatch += "^.*" + regexp.QuoteMeta(trigger)
	case CommandTriggerRegex:
		cmdMatch += trigger
	case CommandTriggerExact:
		cmdMatch += "^" + regexp.QuoteMeta(trigger) + "$"
	default:
		panic(fmt.Sprintf("Unknown TriggerType %s", cmd.TriggerType))
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
	args[0] = msg[:idx[1]]
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

type CacheKey int

const (
	CacheKeyCommands CacheKey = iota
)

func BotCachedGetCommandsWithMessageTriggers(gs *dstate.GuildState, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := gs.UserCacheFetch(true, CacheKeyCommands, func() (interface{}, error) {
		return models.CustomCommands(qm.Where("guild_id = ?", gs.Guild.ID), qm.Load("Group")).AllG(ctx)
	})

	if err != nil {
		return nil, err
	}

	return v.(models.CustomCommandSlice), nil
}
