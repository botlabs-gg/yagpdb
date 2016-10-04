package customcommands

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"regexp"
	"strings"
	"unicode/utf8"
)

func HandleMessageCreate(s *discordgo.Session, evt *discordgo.MessageCreate, client *redis.Client) {
	if s.State.User == nil || s.State.User.ID == evt.Author.ID {
		return // ignore ourselves
	}

	if evt.Author.Bot {
		return // ignore bots
	}

	channel, err := s.State.Channel(evt.ChannelID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving channel from state")
		return
	}

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

	data := map[string]interface{}{
		"User":    evt.Author,
		"user":    evt.Author,
		"Channel": channel,
	}

	out, err := common.ParseExecuteTemplate(matched.Response, data)
	if err != nil {
		out = "Error executing custom command:" + err.Error() + " (Contact support on the yagpdb support server)"
	}

	if utf8.RuneCountInString(out) > 2000 {
		out = "Custom command response was longer than 2k (contact an admin on the server...)"
	}

	_, err = s.ChannelMessageSend(evt.ChannelID, out)
	if err != nil {
		log.WithError(err).Error("Failed sending message")
	}
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
		startsWith = globalPrefix + trigger

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
