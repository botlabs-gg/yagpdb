package notifications

import (
	"fmt"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
	log "github.com/sirupsen/logrus"
	"math/rand"
)

func HandleGuildMemberAdd(evt *eventsystem.EventData) {
	config := GetConfig(evt.GuildMemberAdd.GuildID)
	if !config.JoinServerEnabled && !config.JoinDMEnabled {
		return
	}

	gs := bot.State.Guild(true, evt.GuildMemberAdd.GuildID)

	client := bot.ContextRedis(evt.Context())

	// Beware of the pyramid and its curses
	if config.JoinDMEnabled && !evt.GuildMemberAdd.User.Bot {

		msg, err := templates.NewContext(bot.State.User(true).User, gs, nil, evt.GuildMemberAdd.Member).Execute(client, config.JoinDMMsg)
		if err != nil {
			log.WithError(err).WithField("guild", gs.ID()).Warn("Failed parsing/executing dm template")
		} else {
			err = bot.SendDM(evt.GuildMemberAdd.User.ID, msg)
			if err != nil {
				log.WithError(err).WithField("guild", gs.ID()).Error("Failed sending join dm")
			}
		}
	}

	if config.JoinServerEnabled && len(config.JoinServerMsgs) > 0 {
		channel := GetChannel(gs, config.JoinServerChannelInt())
		if channel == 0 {
			return
		}
		chanMsg := config.JoinServerMsgs[rand.Intn(len(config.JoinServerMsgs))]
		msg, err := templates.NewContext(bot.State.User(true).User, gs, nil, evt.GuildMemberAdd.Member).Execute(client, chanMsg)
		if err != nil {
			log.WithError(err).WithField("guild", gs.ID()).Warn("Failed parsing/executing join template")
		} else {
			bot.QueueMergedMessage(channel, msg)
		}
	}
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) {
	config := GetConfig(evt.GuildMemberRemove.GuildID)
	if !config.LeaveEnabled {
		return
	}

	gs := bot.State.Guild(true, evt.GuildMemberRemove.GuildID)
	if gs == nil {
		return
	}

	channel := GetChannel(gs, config.LeaveChannelInt())
	if channel == 0 {
		return
	}

	if len(config.LeaveMsgs) == 0 {
		return
	}
	chanMsg := config.LeaveMsgs[rand.Intn(len(config.LeaveMsgs))]

	client := bot.ContextRedis(evt.Context())

	msg, err := templates.NewContext(bot.State.User(true).User, gs, nil, evt.GuildMemberRemove.Member).Execute(client, chanMsg)
	if err != nil {
		log.WithError(err).WithField("guild", gs.ID()).Warn("Failed parsing/executing leave template")
		return
	}

	bot.QueueMergedMessage(channel, msg)
}

func HandleChannelUpdate(evt *eventsystem.EventData) {
	cu := evt.ChannelUpdate

	curChannel := bot.State.Channel(true, cu.ID)
	if curChannel == nil {
		return
	}

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
	if config.TopicChannelInt() != 0 {
		c := curChannel.Guild.Channel(true, config.TopicChannelInt())
		if c != nil {
			topicChannel = c.ID()
		}
	}

	_, err := common.BotSession.ChannelMessageSend(topicChannel, common.EscapeSpecialMentions(fmt.Sprintf("Topic in channel <#%d> changed to **%s**", cu.ID, cu.Topic)))
	if err != nil {
		log.WithError(err).WithField("guild", cu.GuildID).Warn("Failed sending topic change message")
	}
}

// GetChannel makes sure the channel is in the guild, if not it returns no channel
func GetChannel(guild *dstate.GuildState, channel int64) int64 {
	c := guild.Channel(true, channel)
	if c == nil {
		return 0
	}

	return c.ID()
}
