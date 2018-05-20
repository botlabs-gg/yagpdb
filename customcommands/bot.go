package customcommands

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/mediocregopher/radix.v2/redis"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

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
		ccs, _, err := GetCommands(data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), data.GS.ID())
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

		return fmt.Sprintf("%s: `%s` - Case sensitive trigger: `%t` ```\n%s\n```", cc.TriggerType, cc.Trigger, cc.CaseSensitive, strings.Join(cc.Responses, "```\n```")), nil

	},
}

func FindCommands(ccs []*CustomCommand, data *dcmd.Data) (foundCCS []*CustomCommand, provided bool) {
	foundCCS = make([]*CustomCommand, 0, len(ccs))

	provided = true
	if data.Args[0].Value != nil {
		// Find by ID
		id := data.Args[0].Int()
		for _, v := range ccs {
			if v.ID == id {
				foundCCS = append(foundCCS, v)
			}
		}
	} else if data.Args[1].Value != nil {
		// Find by name
		name := data.Args[1].Str()
		for _, v := range ccs {
			if strings.EqualFold(name, v.Trigger) {
				foundCCS = append(foundCCS, v)
			}
		}
	} else {
		provided = false
	}

	return
}

func StringCommands(ccs []*CustomCommand) string {
	out := ""
	for _, cc := range ccs {
		out += fmt.Sprintf("%s: `%s`\n", cc.Trigger, cc.TriggerType.String())
	}

	return out
}

func shouldIgnoreChannel(evt *discordgo.MessageCreate, userID int64, cState *dstate.ChannelState) bool {
	if cState == nil {
		log.Warn("Channel not found in state")
		return true
	}

	if userID == evt.Author.ID || evt.Author.Bot || cState.IsPrivate() {
		return true
	}

	channelPerms, err := cState.Guild.MemberPermissions(true, cState.ID(), userID)
	if err != nil {
		log.WithFields(log.Fields{"guild": cState.Guild.ID(), "channel": cState.ID()}).WithError(err).Error("Failed checking channel perms")
		return true
	}

	if channelPerms&discordgo.PermissionSendMessages == 0 {
		return true
	}

	// Passed all checks, custom commands should not ignore this channel
	return false
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	client := bot.ContextRedis(evt.Context())

	botUser := bot.State.User(true)
	cs := bot.State.Channel(true, evt.MessageCreate.ChannelID)

	if shouldIgnoreChannel(evt.MessageCreate, botUser.ID, cs) {
		return
	}

	cmds, _, err := GetCommands(client, cs.Guild.ID())
	if err != nil {
		log.WithError(err).WithField("guild", cs.Guild.ID()).Error("Failed retrieving comamnds")
		return
	}

	if len(cmds) < 1 {
		return
	}

	prefix, err := commands.GetCommandPrefix(client, cs.Guild.ID())
	if err != nil {
		log.WithError(err).Error("Failed getting prefix")
		return
	}

	member, err := bot.GetMember(cs.Guild.ID(), evt.MessageCreate.Author.ID)
	if err != nil {
		return
	}

	var matched *CustomCommand
	var stripped string
	var args []string
	for _, cmd := range cmds {
		if !cmd.RunsInChannel(evt.MessageCreate.ChannelID) || !cmd.RunsForUser(member) {
			continue
		}

		if m, s, a := CheckMatch(prefix, cmd, evt.MessageCreate.Content); m {
			matched = cmd
			stripped = s
			args = a
			break
		}
	}

	if matched == nil || len(matched.Responses) == 0 {
		return
	}

	channel := cs.Copy(true, true)
	log.WithFields(log.Fields{
		"trigger":      matched.Trigger,
		"trigger_type": matched.TriggerType,
		"guild":        channel.GuildID,
		"channel_name": channel.Name,
	}).Info("Custom command triggered")

	out, delTrigger, delResponse, err := ExecuteCustomCommand(matched, args, stripped, client, bot.ContextSession(evt.Context()), evt.MessageCreate)
	if err != nil {
		log.WithField("guild", channel.GuildID).WithError(err).Error("Error executing custom command")
		out += "\nAn error caused the execution of the custom command template to stop:\n"
		out += common.EscapeSpecialMentions(err.Error())
	}

	if strings.TrimSpace(out) != "" {
		m, err := common.BotSession.ChannelMessageSend(evt.MessageCreate.ChannelID, out)
		if err != nil {
			log.WithError(err).Error("Failed sending message")
		} else {
			if delResponse {
				go common.DelayedMessageDelete(common.BotSession, time.Second*10, m.ChannelID, m.ID)
			}
		}
	}

	if delTrigger {
		go common.DelayedMessageDelete(common.BotSession, time.Second*10, evt.MessageCreate.ChannelID, evt.MessageCreate.ID)
	}
}

func ExecuteCustomCommand(cmd *CustomCommand, cmdArgs []string, stripped string, client *redis.Client, s *discordgo.Session, m *discordgo.MessageCreate) (resp string, delTrigger bool, delResponse bool, err error) {

	cs := bot.State.Channel(true, m.ChannelID)
	member, err := bot.GetMember(cs.Guild.ID(), m.Author.ID)
	if err != nil {
		err = err
		return
	}

	tmplCtx := templates.NewContext(bot.State.User(true).User, cs.Guild, cs, member)
	tmplCtx.Redis = client
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

	chanMsg := cmd.Responses[rand.Intn(len(cmd.Responses))]
	out, err := tmplCtx.Execute(client, chanMsg)

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command response was longer than 2k (contact an admin on the server...)"
	}
	resp = out

	delTrigger = tmplCtx.DelTrigger
	delResponse = tmplCtx.DelResponse

	return
}

// CheckMatch returns true if the given cmd matches, as well as the arguments
// following the command trigger (arg 0 being the message up to, and including,
// the trigger).
func CheckMatch(globalPrefix string, cmd *CustomCommand, msg string) (match bool, stripped string, args []string) {
	trigger := cmd.Trigger

	cmdMatch := "(?m)"
	if !cmd.CaseSensitive {
		cmdMatch += "(?i)"
	}
	switch cmd.TriggerType {
	case CommandTriggerCommand:
		cmdMatch += "^" + regexp.QuoteMeta(globalPrefix+trigger) + "($|[[:space:]])"
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

	re, err := regexp.Compile(cmdMatch)
	if err != nil {
		log.WithError(err).Errorf("Failed compiling regex '%s' (%s)", cmdMatch, cmd.TriggerType)
		return false, "", nil
	}

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
	if !cmd.CaseSensitive && cmd.TriggerType != CommandTriggerRegex {
		stripped = strings.ToLower(msg)
	}
	switch cmd.TriggerType {
	case CommandTriggerStartsWith:
		stripped = args[0]
	case CommandTriggerCommand:
		stripped = strings.TrimSpace(stripped)
	case CommandTriggerContains:
	case CommandTriggerRegex:
		break
	case CommandTriggerExact:
		stripped = ""
	default:
		panic(fmt.Sprintf("Unknown TriggerType %s", cmd.TriggerType))
	}

	match = true
	return
}
