package customcommands

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"log"
	"regexp"
	"strings"
)

func HandleMessageCreate(s *discordgo.Session, evt *discordgo.MessageCreate) {
	if s.State.User == nil || s.State.User.ID == evt.Author.ID {
		return // ignore ourselves
	}

	client, redisErr := bot.RedisPool.Get()
	if redisErr != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer bot.RedisPool.CarefullyPut(client, &redisErr)

	channel, err := s.State.Channel(evt.ChannelID)
	if err != nil {
		log.Println("Failed getting channel from state", err)
		return
	}

	cmds, _, err := GetCommands(client, channel.GuildID)
	if err != nil {
		log.Println("Failed getting commands", err)
		return
	}

	if len(cmds) < 1 {
		return
	}

	cmdConfig := commands.GetConfig(client, channel.GuildID)

	var matched *CustomCommand
	for _, cmd := range cmds {
		if CheckMatch(cmdConfig.Prefix, cmd, evt.Content) {
			matched = cmd
			break
		}
	}

	if matched == nil || matched.Response == "" {
		return
	}

	_, err = s.ChannelMessageSend(evt.ChannelID, matched.Response)
	if err != nil {
		log.Println("Failed sending message", err)
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
	case CommandTriggerStartsWith:
		startsWith = trigger
	case CommandTriggerCommand:
		startsWith = globalPrefix + " " + trigger
		// Special triggertypes
	case CommandTriggerContains:
		return strings.Contains(msg, trigger)
	case CommandTriggerRegex:
		ok, err := regexp.Match(cmd.Trigger, []byte(msg))
		if err != nil {
			log.Println("Failed compiling regex", err)
		}
		return ok
	case CommandTriggerExact:
		return msg == trigger
	}

	return strings.Index(msg, startsWith+"") == 0
}
