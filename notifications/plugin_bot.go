package notifications

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"log"
	"sync"
	"text/template"
)

var (
	// This is just a workaround, should probably find a better way to do this
	Topics     map[string]string = make(map[string]string)
	TopicsLock sync.Mutex
)

func HandleGuildCreate(s *discordgo.Session, evt *discordgo.GuildCreate) {
	TopicsLock.Lock()
	for _, channel := range evt.Channels {
		Topics[channel.ID] = channel.Topic
	}
	TopicsLock.Unlock()
}

func HandleGuildMemberAdd(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
	client, redisErr := bot.RedisPool.Get()
	if redisErr != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer bot.RedisPool.CarefullyPut(client, &redisErr)

	guild, err := s.State.Guild(evt.GuildID)
	if err != nil {
		log.Println("!Guild not found in state!", evt.GuildID, err)
		return // We can't process this then
	}

	templateData := map[string]interface{}{
		"user":  evt.User,
		"guild": guild,
	}
	config := GetConfig(client, evt.GuildID)

	// Beware of the spaghetti
	if config.JoinDMEnabled {

		msg, err := ParseExecuteTemplate(config.JoinDMMsg, templateData)
		if err != nil {
			log.Println("Failed parsing/executing dm template", guild.ID, err)
		} else {
			privateChannel, err := bot.GetCreatePrivateChannel(s, evt.User.ID)
			if err != nil {
				log.Println("Failed retrieving private channel", evt.GuildID, evt.User.ID, err)
			} else {
				_, err := s.ChannelMessageSend(privateChannel.ID, msg)
				if err != nil {
					log.Println("Failed sending join msg", evt.GuildID, err)
				}
			}
		}
	}

	if config.JoinServerEnabled {
		channel := GetChannel(guild, config.JoinServerChannel)
		msg, err := ParseExecuteTemplate(config.JoinServerMsg, templateData)
		if err != nil {
			log.Println("Failed parsing/executing dm template", guild.ID, err)
		} else {
			_, err = s.ChannelMessageSend(channel, msg)
			if err != nil {
				log.Println("Failed sending join message to server channel", guild.ID, err)
			}
		}
	}
}

func HandleGuildMemberRemove(s *discordgo.Session, evt *discordgo.GuildMemberRemove) {
	client, redisErr := bot.RedisPool.Get()
	if redisErr != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer bot.RedisPool.CarefullyPut(client, &redisErr)

	guild, err := s.State.Guild(evt.GuildID)
	if err != nil {
		log.Println("!Guild not found in state!", evt.GuildID, err)
		return // We can't process this then
	}

	templateData := map[string]interface{}{
		"user":  evt.User,
		"guild": guild,
	}
	config := GetConfig(client, evt.GuildID)

	if !config.LeaveEnabled {
		return
	}

	channel := GetChannel(guild, config.LeaveChannel)
	msg, err := ParseExecuteTemplate(config.LeaveMsg, templateData)
	if err != nil {
		log.Println("Failed parsing/executing leave template", guild.ID, err)
		return
	}

	_, err = s.ChannelMessageSend(channel, msg)
	if err != nil {
		log.Println("Failed sending leave message", guild.ID, err)
	}
}

func HandleChannelUpdate(s *discordgo.Session, evt *discordgo.ChannelUpdate) {
	TopicsLock.Lock()
	curTopic := Topics[evt.ID]
	Topics[evt.ID] = evt.Topic
	TopicsLock.Unlock()

	if curTopic == evt.Topic {
		return
	}

	client, redisErr := bot.RedisPool.Get()
	if redisErr != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer bot.RedisPool.CarefullyPut(client, &redisErr)

	config := GetConfig(client, evt.GuildID)
	if config.TopicEnabled {
		guild, err := s.State.Guild(evt.GuildID)
		if err != nil {
			log.Println("Failed getting guild from state", err)
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

func ParseExecuteTemplate(tmplSource string, data interface{}) (string, error) {
	parsed, err := template.New("").Parse(tmplSource)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = parsed.Execute(&buf, data)
	return buf.String(), err
}
