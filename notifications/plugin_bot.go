package notifications

import (
	"fmt"
	"math/rand"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandlerFirst(p, HandleChannelUpdate, eventsystem.EventChannelUpdate)
}

func HandleGuildMemberAdd(evtData *eventsystem.EventData) (retry bool, err error) {
	evt := evtData.GuildMemberAdd()

	config, err := GetConfig(evt.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.JoinServerEnabled && !config.JoinDMEnabled {
		return
	}

	if (!config.JoinDMEnabled || evt.User.Bot) && !config.JoinServerEnabled {
		return
	}

	gs := evtData.GS
	ms := dstate.MemberStateFromMember(evt.Member)
	ms.GuildID = evt.GuildID

	// Beware of the pyramid and its curses
	if config.JoinDMEnabled && !evt.User.Bot {
		cid, err := common.BotSession.UserChannelCreate(evt.User.ID)
		if err != nil {
			if bot.CheckDiscordErrRetry(err) {
				return true, errors.WithStackIf(err)
			}

			logger.WithError(err).WithField("user", evt.User.ID).Error("Failed retrieving user channel")
		} else {
			thinCState := &dstate.ChannelState{
				ID:      cid.ID,
				Name:    evt.User.Username,
				Type:    discordgo.ChannelTypeDM,
				GuildID: gs.ID,
			}

			go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "posted_join_server_msg")

			if sendTemplate(gs, thinCState, config.JoinDMMsg, ms, "join dm", false, true) {
				return true, nil
			}
		}
	}

	if config.JoinServerEnabled && len(config.JoinServerMsgs) > 0 {
		channel := gs.GetChannel(config.JoinServerChannelInt())
		if channel == nil {
			return
		}

		go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "posted_join_server_dm")

		chanMsg := config.JoinServerMsgs[rand.Intn(len(config.JoinServerMsgs))]
		if sendTemplate(gs, channel, chanMsg, ms, "join server msg", config.CensorInvites, true) {
			return true, nil
		}
	}

	return false, nil
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) (retry bool, err error) {
	memberRemove := evt.GuildMemberRemove()

	config, err := GetConfig(memberRemove.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.LeaveEnabled || len(config.LeaveMsgs) == 0 {
		return
	}

	gs := bot.State.GetGuild(memberRemove.GuildID)
	if gs == nil {
		return
	}

	channel := gs.GetChannel(config.LeaveChannelInt())
	if channel == nil {
		return
	}

	ms := dstate.MemberStateFromMember(memberRemove.Member)

	chanMsg := config.LeaveMsgs[rand.Intn(len(config.LeaveMsgs))]

	go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "posted_leave_server_msg")

	if sendTemplate(gs, channel, chanMsg, ms, "leave", config.CensorInvites, false) {
		return true, nil
	}

	return false, nil
}

// sendTemplate parses and executes the provided template, returns wether an error occured that we can retry from (temporary network failures and the like)
func sendTemplate(gs *dstate.GuildSet, cs *dstate.ChannelState, tmpl string, ms *dstate.MemberState, name string, censorInvites bool, enableSendDM bool) bool {
	ctx := templates.NewContext(gs, cs, ms)
	ctx.CurrentFrame.SendResponseInDM = cs.Type == discordgo.ChannelTypeDM
	ctx.IsExecedByLeaveMessage = !enableSendDM

	// since were changing the fields, we need a copy
	msCop := *ms
	ms = &msCop

	ctx.Data["RealUsername"] = ms.User.Username
	if censorInvites {
		newUsername := common.ReplaceServerInvites(ms.User.Username, gs.ID, "[removed-server-invite]")
		if newUsername != ms.User.Username {
			ms.User.Username = newUsername + fmt.Sprintf("(user ID: %d)", ms.User.ID)
			ctx.Data["UsernameHasInvite"] = true
		}
	}

	// Disable sendDM if needed
	disableFuncs := []string{}
	if !enableSendDM {
		disableFuncs = []string{"sendDM"}
	}
	ctx.DisabledContextFuncs = disableFuncs

	msg, err := ctx.Execute(tmpl)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).Warnf("Failed parsing/executing %s template", name)
		return false
	}

	msg = strings.TrimSpace(msg)
	if msg == "" {
		return false
	}

	var m *discordgo.Message
	if cs.Type == discordgo.ChannelTypeDM {
		msg = "DM sent from server **" + gs.Name + "** (ID: " + discordgo.StrID(gs.ID) + ")\n" + msg
		m, err = common.BotSession.ChannelMessageSend(cs.ID, msg)
	} else {
		if len(ctx.CurrentFrame.AddResponseReactionNames) > 0 || ctx.CurrentFrame.DelResponse {
			m, err = common.BotSession.ChannelMessageSendComplex(cs.ID, ctx.MessageSend(msg))
			if err == nil && ctx.CurrentFrame.DelResponse {
				templates.MaybeScheduledDeleteMessage(gs.ID, cs.ID, m.ID, ctx.CurrentFrame.DelResponseDelay)
			}
		} else {
			send := ctx.MessageSend("")
			bot.QueueMergedMessage(cs.ID, msg, send.AllowedMentions)
		}
	}

	if err == nil && m != nil && len(ctx.CurrentFrame.AddResponseReactionNames) > 0 {
		go func(frame *templates.ContextFrame) {
			for _, v := range frame.AddResponseReactionNames {
				common.BotSession.MessageReactionAdd(m.ChannelID, m.ID, v)
			}
		}(ctx.CurrentFrame)
	}

	if err != nil {
		l := logger.WithError(err).WithField("guild", gs.ID)
		if common.IsDiscordErr(err, discordgo.ErrCodeCannotSendMessagesToThisUser) {
			l.Warn("Failed sending " + name)
		} else {
			l.Error("Failed sending " + name)
		}
	}

	return bot.CheckDiscordErrRetry(err)
}

func HandleChannelUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	cu := evt.ChannelUpdate()

	gs := bot.State.GetGuild(cu.GuildID)
	if gs == nil {
		return
	}

	curChannel := gs.GetChannel(cu.ID)
	if curChannel == nil {
		return
	}

	oldTopic := curChannel.Topic

	if oldTopic == cu.Topic {
		return
	}

	config, err := GetConfig(cu.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.TopicEnabled {
		return
	}

	topicChannel := cu.Channel.ID
	if config.TopicChannelInt() != 0 {
		c := gs.GetChannel(config.TopicChannelInt())
		if c != nil {
			topicChannel = c.ID
		}
	}

	go analytics.RecordActiveUnit(cu.GuildID, &Plugin{}, "posted_topic_change")

	go func() {
		_, err := common.BotSession.ChannelMessageSend(topicChannel, fmt.Sprintf("Topic in channel <#%d> changed to **%s**", cu.ID, cu.Topic))
		if err != nil {
			logger.WithError(err).WithField("guild", cu.GuildID).Warn("Failed sending topic change message")
		}
	}()

	return false, nil
}
