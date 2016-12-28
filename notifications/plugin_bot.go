package notifications

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
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
	config := GetConfig(evt.GuildID)
	if !config.JoinServerEnabled && !config.JoinDMEnabled {
		return
	}

	gs := bot.State.Guild(true, evt.GuildID)
	templateData := createTemplateData(gs, evt.User)

	// Beware of the pyramid and its curses
	if config.JoinDMEnabled {

		msg, err := common.ParseExecuteTemplate(config.JoinDMMsg, templateData)
		if err != nil {
			log.WithError(err).WithField("guild", gs.ID()).Error("Failed parsing/executing dm template")
		} else {
			privateChannel, err := bot.GetCreatePrivateChannel(s, evt.User.ID)
			if err != nil {
				log.WithError(err).WithField("guild", gs.ID()).Error("Failed retrieving private channel")
			} else {
				_, err := s.ChannelMessageSend(privateChannel.ID, msg)
				if err != nil {
					log.WithError(err).WithField("guild", gs.ID()).Error("Failed sending join message")
				}
			}
		}
	}

	if config.JoinServerEnabled {
		channel := GetChannel(gs, config.JoinServerChannel)
		msg, err := common.ParseExecuteTemplate(config.JoinServerMsg, templateData)
		if err != nil {
			log.WithError(err).WithField("guild", gs.ID()).Error("Failed parsing/executing join template")
		} else {
			bot.QueueMergedMessage(channel, msg)
		}
	}
}

func HandleGuildMemberRemove(s *discordgo.Session, evt *discordgo.GuildMemberRemove, client *redis.Client) {
	config := GetConfig(evt.GuildID)
	if !config.LeaveEnabled {
		return
	}

	gs := bot.State.Guild(true, evt.GuildID)

	templateData := createTemplateData(gs, evt.User)
	channel := GetChannel(gs, config.LeaveChannel)
	msg, err := common.ParseExecuteTemplate(config.LeaveMsg, templateData)
	if err != nil {
		log.WithError(err).WithField("guild", gs.ID()).Error("Failed parsing/executing leave template")
		return
	}

	bot.QueueMergedMessage(channel, msg)
}

func createTemplateData(gs *dstate.GuildState, user *discordgo.User) map[string]interface{} {
	gCopy := gs.LightCopy(true)

	templateData := map[string]interface{}{
		"user":   user, // Deprecated
		"User":   user,
		"guild":  gCopy, // Deprecated
		"Guild":  gCopy,
		"Server": gCopy,
	}

	return templateData
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
	if !config.TopicEnabled {
		return
	}

	gs := bot.State.Guild(true, evt.GuildID)
	topicChannel := evt.Channel.ID

	if config.TopicChannel != "" {
		c := gs.Channel(true, config.TopicChannel)
		if c != nil {
			topicChannel = c.ID()
		}
	}

	_, err := s.ChannelMessageSend(topicChannel, fmt.Sprintf("Topic in channel **%s** changed to **%s**", evt.Name, evt.Topic))
	if err != nil {
		log.WithError(err).WithField("guild", evt.GuildID).Error("Failed sending topic change message")
	}
}

// GetChannel makes sure the channel is in the guild, if not it returns the default channel (same as guildid)
func GetChannel(guild *dstate.GuildState, channel string) string {
	c := guild.Channel(true, channel)
	if c == nil {
		return guild.ID()
	}

	return c.ID()
}
