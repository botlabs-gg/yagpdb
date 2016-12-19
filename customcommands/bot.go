package customcommands

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"
)

func HandleMessageCreate(s *discordgo.Session, evt *discordgo.MessageCreate, client *redis.Client) {
	if s.State.User == nil || s.State.User.ID == evt.Author.ID {
		return // ignore ourselves
	}

	if evt.Author.Bot {
		return // ignore bots
	}

	channel := common.MustGetChannel(evt.ChannelID)

	cmds, _, err := GetCommands(client, channel.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed getting comamnds")
		return
	}

	if len(cmds) < 1 {
		return
	}

	prefix, err := commands.GetCommandPrefix(client, channel.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed getting prefix")
		return
	}

	var matched *CustomCommand
	for _, cmd := range cmds {
		if CheckMatch(prefix, cmd, evt.Content) {
			matched = cmd
			break
		}
	}

	if matched == nil || matched.Response == "" {
		return
	}

	log.WithFields(log.Fields{
		"trigger":      matched.Trigger,
		"trigger_type": matched.TriggerType,
		"guild":        channel.GuildID,
		"channel_name": channel.Name,
	}).Info("Custom command triggered")

	out, err := ExecuteCustomCommand(matched, client, s, evt)
	if err != nil {
		if out == "" {
			out += err.Error()
		}
		log.WithField("guild", channel.GuildID).WithError(err).Error("Error executing custom command")
		out += "\nAn error caused the execution of the custom command template to stop"
	}

	if out != "" {
		_, err = s.ChannelMessageSend(evt.ChannelID, out)
		if err != nil {
			log.WithError(err).Error("Failed sending message")
		}
	}
}

func ExecuteCustomCommand(cmd *CustomCommand, client *redis.Client, s *discordgo.Session, m *discordgo.MessageCreate) (string, error) {
	channel := common.MustGetChannel(m.ChannelID)

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
	for _, command := range commands.CommandSystem.Commands {
		if !command.CheckMatch(cmdLine, commandsystem.CommandSourcePrefix, m, s) {
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

	parsed, err := cast.ParseCommand(cmdLine, m, s)
	if err != nil {
		return "", err
	}

	parsed.Channel = common.MustGetChannel(m.ChannelID)
	parsed.Guild = common.MustGetGuild(parsed.Channel.GuildID)

	resp, err := cast.RunFunc(parsed, client, m)

	switch v := resp.(type) {
	case error:
		return "Error: " + v.Error(), err
	case string:
		return v, err
	case *discordgo.MessageEmbed:
		return common.FallbackEmbed(v), err
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
		ok, err := regexp.Match(cmd.Trigger, []byte(msg))
		if err != nil {
			log.WithError(err).Error("Failed compiling regex")
		}

		return ok
	case CommandTriggerExact:
		return msg == trigger
	}

	return strings.Index(msg, startsWith+"") == 0
}
