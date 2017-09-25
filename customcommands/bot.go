package customcommands

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/mediocregopher/radix.v2/redis"
	"regexp"
	"strings"
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
	var stripped string
	for _, cmd := range cmds {
		if m, s := CheckMatch(prefix, cmd, evt.MessageCreate.Content); m {
			matched = cmd
			stripped = s
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

	out, err := ExecuteCustomCommand(matched, stripped, client, bot.ContextSession(evt.Context()), evt.MessageCreate)
	if err != nil {
		if out == "" {
			out += err.Error()
		}
		log.WithField("guild", channel.GuildID).WithError(err).Error("Error executing custom command")
		out += "\nAn error caused the execution of the custom command template to stop"
	}

	if out != "" {
		_, err = common.BotSession.ChannelMessageSend(evt.MessageCreate.ChannelID, common.EscapeSpecialMentions(out))
		if err != nil {
			log.WithError(err).Error("Failed sending message")
		}
	}
}

func ExecuteCustomCommand(cmd *CustomCommand, stripped string, client *redis.Client, s *discordgo.Session, m *discordgo.MessageCreate) (string, error) {

	cs := bot.State.Channel(true, m.ChannelID)
	ms := cs.Guild.Member(true, m.Author.ID)
	tmplCtx := templates.NewContext(bot.State.User(true).User, cs.Guild, cs, ms.Member)
	tmplCtx.Redis = client
	tmplCtx.Msg = m.Message

	args := commandsystem.ReadArgs(m.Content)
	argsStr := make([]string, len(args))
	for k, v := range args {
		argsStr[k] = v.Raw.Str
	}

	tmplCtx.Data["Args"] = argsStr
	tmplCtx.Data["StrippedMsg"] = stripped

	out, err := tmplCtx.Execute(client, cmd.Response)

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command response was longer than 2k (contact an admin on the server...)"
	}

	return out, err
}

func CheckMatch(globalPrefix string, cmd *CustomCommand, msg string) (match bool, stripped string) {
	// set to globalprefix+" "+localprefix for command, and just local prefix for startwith
	startsWith := ""

	trigger := cmd.Trigger

	if !cmd.CaseSensitive && cmd.TriggerType != CommandTriggerRegex {
		msg = strings.ToLower(msg)
		trigger = strings.ToLower(cmd.Trigger)
		globalPrefix = strings.ToLower(globalPrefix)
	}

	switch cmd.TriggerType {
	// Simpler triggers
	case CommandTriggerStartsWith:
		startsWith = trigger
	case CommandTriggerCommand:
		split := strings.SplitN(msg, " ", 2)
		if len(split) > 0 && split[0] == globalPrefix+trigger {
			if len(split) > 1 {
				stripped = strings.TrimSpace(split[1])
			}

			return true, stripped
		} else {
			return false, ""
		}
	// Special trigger types
	case CommandTriggerContains:
		return strings.Contains(msg, trigger), msg
	case CommandTriggerRegex:
		rTrigger := cmd.Trigger
		if !cmd.CaseSensitive && !strings.HasPrefix(rTrigger, "(?i)") {
			rTrigger = "(?i)" + rTrigger
		}
		ok, err := regexp.Match(rTrigger, []byte(msg))
		if err != nil {
			log.WithError(err).Error("Failed compiling regex")
		}

		return ok, msg
	case CommandTriggerExact:
		return msg == trigger, ""
	}

	if strings.HasPrefix(msg, startsWith) {
		stripped = msg[:len(startsWith)]
		return true, stripped
	}

	return false, ""
}
