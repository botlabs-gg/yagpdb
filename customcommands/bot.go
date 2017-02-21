package customcommands

import (
	"context"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"
)

func shouldIgnoreChannel(evt *discordgo.MessageCreate, userID string, cState *dstate.ChannelState) bool {
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

	var matched *CustomCommand
	for _, cmd := range cmds {
		if CheckMatch(prefix, cmd, evt.MessageCreate.Content) {
			matched = cmd
			break
		}
	}

	if matched == nil || matched.Response == "" {
		return
	}

	channel := cs.Copy(true, true)
	log.WithFields(log.Fields{
		"trigger":      matched.Trigger,
		"trigger_type": matched.TriggerType,
		"guild":        channel.GuildID,
		"channel_name": channel.Name,
	}).Info("Custom command triggered")

	out, err := ExecuteCustomCommand(matched, client, bot.ContextSession(evt.Context()), evt.MessageCreate)
	if err != nil {
		if out == "" {
			out += err.Error()
		}
		log.WithField("guild", channel.GuildID).WithError(err).Error("Error executing custom command")
		out += "\nAn error caused the execution of the custom command template to stop"
	}

	if out != "" {
		_, err = common.BotSession.ChannelMessageSend(evt.MessageCreate.ChannelID, common.EscapeEveryoneMention(out))
		if err != nil {
			log.WithError(err).Error("Failed sending message")
		}
	}
}

func ExecuteCustomCommand(cmd *CustomCommand, client *redis.Client, s *discordgo.Session, m *discordgo.MessageCreate) (string, error) {
	cs := bot.State.Channel(true, m.ChannelID)
	channel := cs.Copy(true, true)

	data := map[string]interface{}{
		"User":    m.Author,
		"user":    m.Author,
		"Channel": channel,
	}

	args := commandsystem.ReadArgs(m.Content)
	argsStr := make([]string, len(args))
	for k, v := range args {
		argsStr[k] = v.Raw.Str
	}
	data["Args"] = argsStr

	execUser, execBot := execCmdFuncs(3, false, client, s, m)

	//out, err := common.ParseExecuteTemplateFM(cmd.Response, data, template.FuncMap{"exec": execUser, "execBot": execBot})
	out, err := common.ParseExecuteTemplateFM(cmd.Response, data, template.FuncMap{
		"exec":    execUser,
		"execBot": execBot,
		"shuffle": shuffle,
		"seq":     sequence,
		"joinStr": joinStrings,
	})

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command response was longer than 2k (contact an admin on the server...)"
	}

	return out, err
}

type cmdExecFunc func(cmd string, args ...interface{}) (string, error)

// Returns 2 functions to execute commands in user or bot context with limited about of commands executed
func execCmdFuncs(maxExec int, dryRun bool, client *redis.Client, s *discordgo.Session, m *discordgo.MessageCreate) (userCtxCommandExec cmdExecFunc, botCtxCommandExec cmdExecFunc) {
	execUser := func(cmd string, args ...interface{}) (string, error) {
		if maxExec < 1 {
			return "", errors.New("Max number of commands executed in custom command")
		}
		maxExec -= 1
		return execCmd(dryRun, client, m.Author, s, m, cmd, args...)
	}

	execBot := func(cmd string, args ...interface{}) (string, error) {
		if maxExec < 1 {
			return "", errors.New("Max number of commands executed in custom command")
		}
		maxExec -= 1
		return execCmd(dryRun, client, m.Author, s, m, cmd, args...)
	}

	return execUser, execBot
}

func execCmd(dryRun bool, client *redis.Client, ctx *discordgo.User, s *discordgo.Session, m *discordgo.MessageCreate, cmd string, args ...interface{}) (string, error) {
	cmdLine := cmd

	for _, arg := range args {
		switch t := arg.(type) {
		case string:
			cmdLine += " \"" + t + "\""
		case int:
			cmdLine += strconv.FormatInt(int64(t), 10)
		case int32:
			cmdLine += strconv.FormatInt(int64(t), 10)
		case int64:
			cmdLine += strconv.FormatInt(t, 10)
		case uint:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint8:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint16:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint32:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint64:
			cmdLine += strconv.FormatUint(t, 10)
		case float32:
			cmdLine += strconv.FormatFloat(float64(t), 'E', -1, 32)
		case float64:
			cmdLine += strconv.FormatFloat(t, 'E', -1, 64)
		default:
			return "", errors.New("Unknown type in exec, contact bot owner")
		}
		cmdLine += " "
	}

	log.Info("Custom command is executing a command:", cmdLine)

	var matchedCmd commandsystem.CommandHandler

	triggerData := &commandsystem.TriggerData{
		Session: common.BotSession,
		DState:  bot.State,
		Message: m.Message,
		Source:  commandsystem.SourcePrefix,
	}

	for _, command := range commands.CommandSystem.Commands {
		if !command.CheckMatch(cmdLine, triggerData) {
			continue
		}
		matchedCmd = command
		break
	}

	if matchedCmd == nil {
		return "", errors.New("Couldn't find command")
	}

	cast, ok := matchedCmd.(*commands.CustomCommand)
	if !ok {
		return "", errors.New("Unsopported command")
	}

	// Do not actually execute the command if it's a dry-run
	if dryRun {
		return "", nil
	}

	parsed, err := cast.ParseCommand(cmdLine, triggerData)
	if err != nil {
		return "", err
	}

	parsed.Source = triggerData.Source

	parsed.Channel = bot.State.Channel(true, m.ChannelID)
	parsed.Guild = parsed.Channel.Guild

	resp, err := cast.Run(parsed.WithContext(context.WithValue(parsed.Context(), commands.CtxKeyRedisClient, client)))

	switch v := resp.(type) {
	case error:
		return "Error: " + v.Error(), nil
	case string:
		return v, nil
	case *discordgo.MessageEmbed:
		return common.FallbackEmbed(v), nil
	}

	return "", err
}

func CheckMatch(globalPrefix string, cmd *CustomCommand, msg string) bool {
	// set to globalprefix+" "+localprefix for command, and just local prefix for startwith
	startsWith := ""

	trigger := cmd.Trigger

	if !cmd.CaseSensitive && cmd.TriggerType != CommandTriggerRegex {
		msg = strings.ToLower(msg)
		trigger = strings.ToLower(cmd.Trigger)
	}

	switch cmd.TriggerType {
	// Simpler triggers
	case CommandTriggerStartsWith:
		startsWith = trigger
	case CommandTriggerCommand:
		return strings.Index(msg, globalPrefix+trigger) == 0 && (len(msg) == len(globalPrefix+trigger) || msg[len(globalPrefix+trigger)] == ' ')

	// Special trigger types
	case CommandTriggerContains:
		return strings.Contains(msg, trigger)
	case CommandTriggerRegex:
		rTrigger := cmd.Trigger
		if !cmd.CaseSensitive && !strings.HasPrefix(rTrigger, "(?i)") {
			rTrigger = "(?i)" + rTrigger
		}
		ok, err := regexp.Match(rTrigger, []byte(msg))
		if err != nil {
			log.WithError(err).Error("Failed compiling regex")
		}

		return ok
	case CommandTriggerExact:
		return msg == trigger
	}

	return strings.Index(msg, startsWith+"") == 0
}
