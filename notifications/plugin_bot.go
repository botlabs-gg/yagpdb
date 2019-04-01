package notifications

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"strings"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(HandleGuildMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandler(HandleGuildMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandlerBefore(HandleChannelUpdate, eventsystem.EventChannelUpdate, bot.StateHandlerPtr)
}

func HandleGuildMemberAdd(evtData *eventsystem.EventData) {
	evt := evtData.GuildMemberAdd()

	config := GetConfig(evt.GuildID)
	if !config.JoinServerEnabled && !config.JoinDMEnabled {
		return
	}

	if (!config.JoinDMEnabled || evt.User.Bot) && !config.JoinServerEnabled {
		return
	}

	gs := bot.State.Guild(true, evt.GuildID)

	ms := dstate.MSFromDGoMember(gs, evt.Member)

	// Beware of the pyramid and its curses
	if config.JoinDMEnabled && !evt.User.Bot {
		cid, err := common.BotSession.UserChannelCreate(evt.User.ID)
		if err != nil {
			log.WithError(err).WithField("user", evt.User.ID).Error("Failed retrieving user channel")
			return
		}

		thinCState := &dstate.ChannelState{
			Owner: gs,
			Guild: gs,
			ID:    cid.ID,
			Name:  evt.User.Username,
			Type:  discordgo.ChannelTypeDM,
		}

		sendTemplate(thinCState, config.JoinDMMsg, ms, "join dm", false)
	}

	if config.JoinServerEnabled && len(config.JoinServerMsgs) > 0 {
		channel := gs.Channel(true, config.JoinServerChannelInt())
		if channel == nil {
			return
		}

		chanMsg := config.JoinServerMsgs[rand.Intn(len(config.JoinServerMsgs))]
		sendTemplate(channel, chanMsg, ms, "join server msg", config.CensorInvites)
	}
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) {
	memberRemove := evt.GuildMemberRemove()

	config := GetConfig(memberRemove.GuildID)
	if !config.LeaveEnabled || len(config.LeaveMsgs) == 0 {
		return
	}

	gs := bot.State.Guild(true, memberRemove.GuildID)
	if gs == nil {
		return
	}

	channel := gs.Channel(true, config.LeaveChannelInt())
	if channel == nil {
		return
	}

	ms := dstate.MSFromDGoMember(gs, memberRemove.Member)

	chanMsg := config.LeaveMsgs[rand.Intn(len(config.LeaveMsgs))]

	sendTemplate(channel, chanMsg, ms, "leave", config.CensorInvites)
}

func sendTemplate(cs *dstate.ChannelState, tmpl string, ms *dstate.MemberState, name string, censorInvites bool) {
	ctx := templates.NewContext(cs.Guild, cs, ms)

	ctx.Data["RealUsername"] = ms.Username
	if censorInvites {
		newUsername := common.ReplaceServerInvites(ms.Username, ms.Guild.ID, "[removed-server-invite]")
		if newUsername != ms.Username {
			ms.Username = newUsername + fmt.Sprintf("(user ID: %d)", ms.ID)
			ctx.Data["UsernameHasInvite"] = true
		}
	}

	msg, err := ctx.Execute(tmpl)

	if err != nil {
		log.WithError(err).WithField("guild", cs.Guild.ID).Warnf("Failed parsing/executing %s template", name)
		return
	}

	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}

	if cs.Type == discordgo.ChannelTypeDM {
		_, err = common.BotSession.ChannelMessageSend(cs.ID, msg)
		if err != nil {
			log.WithError(err).WithField("guild", cs.Guild.ID).Error("Failed sending " + name)
		}
	} else if !ctx.DelResponse {
		bot.QueueMergedMessage(cs.ID, msg)
	} else {
		m, err := common.BotSession.ChannelMessageSend(cs.ID, msg)
		if err == nil && ctx.DelResponse {
			templates.MaybeScheduledDeleteMessage(cs.Guild.ID, cs.ID, m.ID, ctx.DelResponseDelay)
		}
	}
}

func HandleChannelUpdate(evt *eventsystem.EventData) {
	cu := evt.ChannelUpdate()

	curChannel := bot.State.ChannelCopy(true, cu.ID)
	if curChannel == nil {
		return
	}

	oldTopic := curChannel.Topic

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
			topicChannel = c.ID
		}
	}

	go func() {
		_, err := common.BotSession.ChannelMessageSend(topicChannel, common.EscapeSpecialMentions(fmt.Sprintf("Topic in channel <#%d> changed to **%s**", cu.ID, cu.Topic)))
		if err != nil {
			log.WithError(err).WithField("guild", cu.GuildID).Warn("Failed sending topic change message")
		}
	}()
}
