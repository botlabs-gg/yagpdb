package notifications

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"sync"
)

var (
	// The current topics
	Topics     map[string]string = make(map[string]string)
	TopicsLock sync.Mutex
)

func HandleGuildCreate(s *discordgo.Session, evt *discordgo.GuildCreate, client *redis.Client) {
	TopicsLock.Lock()
	for _, channel := range evt.Channels {
		Topics[channel.ID] = channel.Topic
	}
	TopicsLock.Unlock()
}

func HandleGuildMemberAdd(s *discordgo.Session, evt *discordgo.GuildMemberAdd, client *redis.Client) {
	guild, err := s.State.Guild(evt.GuildID)
	if err != nil {
		log.WithError(err).WithField("guild", guild.ID).Error("Guild not found in state")
		return
	}

	templateData := map[string]interface{}{
		"user":   evt.User, // Deprecated
		"User":   evt.User,
		"guild":  guild, // Deprecated
		"Guild":  guild,
		"Server": guild,
	}

	config := GetConfig(evt.GuildID)

	// Beware of the pyramid and its curses
	if config.JoinDMEnabled {

		msg, err := common.ParseExecuteTemplate(config.JoinDMMsg, templateData)
		if err != nil {
			log.WithError(err).WithField("guild", guild.ID).Error("Failed parsing/executing dm template")
		} else {
			privateChannel, err := bot.GetCreatePrivateChannel(s, evt.User.ID)
			if err != nil {
				log.WithError(err).WithField("guild", guild.ID).Error("Failed retrieving private channel")
			} else {
				_, err := s.ChannelMessageSend(privateChannel.ID, msg)
				if err != nil {
					log.WithError(err).WithField("guild", guild.ID).Error("Failed sending join message")
				}
			}
		}
	}

	if config.JoinServerEnabled {
		channel := GetChannel(guild, config.JoinServerChannel)
		msg, err := common.ParseExecuteTemplate(config.JoinServerMsg, templateData)
		if err != nil {
			log.WithError(err).WithField("guild", guild.ID).Error("Failed parsing/executing join template")
		} else {
			bot.QueueMergedMessage(channel, msg)
		}
	}
}

func HandleGuildMemberRemove(s *discordgo.Session, evt *discordgo.GuildMemberRemove, client *redis.Client) {

	guild, err := s.State.Guild(evt.GuildID)
	if err != nil {
		log.WithError(err).Error("Guild not found in state")
		return // We can't process this then
	}

	templateData := map[string]interface{}{
		"user":   evt.User, // Deprecated
		"User":   evt.User,
		"guild":  guild, // Deprecated
		"Guild":  guild,
		"Server": guild,
	}
	config := GetConfig(evt.GuildID)

	if !config.LeaveEnabled {
		return
	}

	channel := GetChannel(guild, config.LeaveChannel)
	msg, err := common.ParseExecuteTemplate(config.LeaveMsg, templateData)
	if err != nil {
		log.WithError(err).WithField("guild", guild.ID).Error("Failed parsing/executing leave template")
		return
	}

	bot.QueueMergedMessage(channel, msg)
}

func HandleChannelUpdate(s *discordgo.Session, evt *discordgo.ChannelUpdate, client *redis.Client) {
	TopicsLock.Lock()
	curTopic := Topics[evt.ID]
	Topics[evt.ID] = evt.Topic
	TopicsLock.Unlock()

	if curTopic == evt.Topic {
		return
	}

	config := GetConfig(evt.GuildID)
	if config.TopicEnabled {
		guild, err := s.State.Guild(evt.GuildID)
		if err != nil {
			log.WithError(err).WithField("guild", guild.ID).Error("Failed getting guild from state")
			return
		}

		topicChannel := evt.Channel

		if config.TopicChannel != "" {
			for _, v := range guild.Channels {
				if v.ID == config.TopicChannel || v.Name == config.TopicChannel {
					topicChannel = v
					break
				}
			}
		}

		_, err = s.ChannelMessageSend(topicChannel.ID, fmt.Sprintf("Topic in channel **%s** changed to **%s**", evt.Name, evt.Topic))
	}
}

func GetChannel(guild *discordgo.Guild, channel string) string {
	for _, v := range guild.Channels {
		if v.ID == channel || v.Name == channel {
			return v.ID
		}
	}

	// Default channel then
	return guild.ID
}
