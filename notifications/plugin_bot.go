package notifications

import (
	"context"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
)

func HandleGuildMemberAdd(ctx context.Context, e interface{}) {
	evt := e.(*discordgo.GuildMemberAdd)
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
			err = bot.SendDM(evt.User.ID, msg)
			if err != nil {
				log.WithError(err).WithField("guild", gs.ID()).Error("Failed sending join dm")
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

func HandleGuildMemberRemove(ctx context.Context, e interface{}) {
	evt := e.(*discordgo.GuildMemberRemove)
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

func HandleChannelUpdate(ctx context.Context, evt interface{}) {
	cu := evt.(*discordgo.ChannelUpdate)

	curChannel := bot.State.Channel(true, cu.ID)
	curChannel.Owner.RLock()
	oldTopic := curChannel.Channel.Topic
	curChannel.Owner.RUnlock()

	if oldTopic == cu.Topic {
		return
	}

	config := GetConfig(cu.GuildID)
	if !config.TopicEnabled {
		return
	}

	topicChannel := cu.Channel.ID
	if config.TopicChannel != "" {
		c := curChannel.Guild.Channel(true, config.TopicChannel)
		if c != nil {
			topicChannel = c.ID()
		}
	}

	_, err := common.BotSession.ChannelMessageSend(topicChannel, fmt.Sprintf("Topic in channel <#%s> changed to **%s**", cu.ID, cu.Topic))
	if err != nil {
		log.WithError(err).WithField("guild", cu.GuildID).Error("Failed sending topic change message")
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
