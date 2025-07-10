package notifications

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/karlseguin/ccache"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandlerFirst(p, HandleChannelUpdate, eventsystem.EventChannelUpdate)

	pubsub.AddHandler("invalidate_notifications_config_cache", handleInvalidateConfigCache, nil)
}

var configCache = ccache.New(ccache.Configure().MaxSize(15000))

func BotCachedGetConfig(guildID int64) (*Config, error) {
	const cacheDuration = 10 * time.Minute

	item, err := configCache.Fetch(cacheKey(guildID), cacheDuration, func() (interface{}, error) {
		return FetchConfig(guildID)
	})
	if err != nil {
		return nil, err
	}
	return item.Value().(*Config), nil
}

func handleInvalidateConfigCache(evt *pubsub.Event) {
	configCache.Delete(cacheKey(evt.TargetGuildInt))
}

func cacheKey(guildID int64) string {
	return discordgo.StrID(guildID)
}

func HandleGuildMemberAdd(evtData *eventsystem.EventData) (retry bool, err error) {
	evt := evtData.GuildMemberAdd()

	config, err := BotCachedGetConfig(evt.GuildID)
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

			if sendTemplate(gs, thinCState, config.JoinDMMsg, ms, "join dm", false, templates.ExecutedFromJoin) {
				return true, nil
			}
		}
	}

	if config.JoinServerEnabled && len(config.JoinServerMsgs) > 0 {
		channel := gs.GetChannel(config.JoinServerChannel)
		if channel == nil {
			return
		}

		go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "posted_join_server_dm")

		chanMsg := config.JoinServerMsgs[rand.Intn(len(config.JoinServerMsgs))]
		if sendTemplate(gs, channel, chanMsg, ms, "join server msg", config.CensorInvites, templates.ExecutedFromJoin) {
			return true, nil
		}
	}

	return false, nil
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) (retry bool, err error) {
	memberRemove := evt.GuildMemberRemove()

	config, err := BotCachedGetConfig(memberRemove.GuildID)
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

	channel := gs.GetChannel(config.LeaveChannel)
	if channel == nil {
		return
	}

	ms := dstate.MemberStateFromMember(memberRemove.Member)

	chanMsg := config.LeaveMsgs[rand.Intn(len(config.LeaveMsgs))]

	go analytics.RecordActiveUnit(gs.ID, &Plugin{}, "posted_leave_server_msg")

	if sendTemplate(gs, channel, chanMsg, ms, "leave", config.CensorInvites, templates.ExecutedFromLeave) {
		return true, nil
	}

	return false, nil
}

// sendTemplate parses and executes the provided template, returns wether an error occured that we can retry from (temporary network failures and the like)
func sendTemplate(gs *dstate.GuildSet, cs *dstate.ChannelState, tmpl string, ms *dstate.MemberState, name string, censorInvites bool, executedFrom templates.ExecutedFromType) bool {
	ctx := templates.NewContext(gs, cs, ms)
	ctx.CurrentFrame.SendResponseInDM = cs.Type == discordgo.ChannelTypeDM
	ctx.ExecutedFrom = executedFrom

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
	if executedFrom == templates.ExecutedFromLeave {
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
		msg = common.ReplaceServerInvites(msg, 0, "[removed-server-invite]")
		msgSend := ctx.MessageSend(msg)
		msgSend.Components = bot.GenerateServerInfoButton(gs.ID)
		m, err = common.BotSession.ChannelMessageSendComplex(cs.ID, msgSend)
	} else {
		if len(ctx.CurrentFrame.AddResponseReactionNames) > 0 || ctx.CurrentFrame.DelResponse || ctx.CurrentFrame.PublishResponse {
			m, err = common.BotSession.ChannelMessageSendComplex(cs.ID, ctx.MessageSend(msg))
			if err == nil && ctx.CurrentFrame.DelResponse {
				templates.MaybeScheduledDeleteMessage(gs.ID, cs.ID, m.ID, ctx.CurrentFrame.DelResponseDelay, "")
			}
		} else {
			send := ctx.MessageSend("")
			bot.QueueMergedMessage(cs.ID, msg, send.AllowedMentions)
		}
	}

	if err == nil && m != nil {
		if len(ctx.CurrentFrame.AddResponseReactionNames) > 0 {
			go func(frame *templates.ContextFrame) {
				for _, v := range frame.AddResponseReactionNames {
					common.BotSession.MessageReactionAdd(m.ChannelID, m.ID, v)
				}
			}(ctx.CurrentFrame)
		}

		if ctx.CurrentFrame.PublishResponse && ctx.CurrentFrame.CS.Type == discordgo.ChannelTypeGuildNews {
			common.BotSession.ChannelMessageCrosspost(m.ChannelID, m.ID)
		}
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

	config, err := BotCachedGetConfig(cu.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.TopicEnabled {
		return
	}

	topicChannel := cu.Channel.ID
	if config.TopicChannel != 0 {
		c := gs.GetChannel(config.TopicChannel)
		if c != nil {
			topicChannel = c.ID
		}
	}

	go analytics.RecordActiveUnit(cu.GuildID, &Plugin{}, "posted_topic_change")

	go func() {
		_, err := common.BotSession.ChannelMessageSend(topicChannel, fmt.Sprintf("Topic in channel <#%d> changed to \x60\x60\x60\n%s\x60\x60\x60", cu.ID, cu.Topic))
		if err != nil {
			logger.WithError(err).WithField("guild", cu.GuildID).Warn("Failed sending topic change message")
		}
	}()

	return false, nil
}
