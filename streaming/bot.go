package streaming

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/mediocregopher/radix/v3"
)

func KeyCurrentlyStreaming(gID int64) string { return "currently_streaming:" + discordgo.StrID(gID) }

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.LimitedConcurrentEventHandler(HandleGuildCreate, 10, time.Millisecond*200), eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLast(p, HandlePresenceUpdate, eventsystem.EventPresenceUpdate)
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)
	pubsub.AddHandler("update_streaming", HandleUpdateStreaming, nil)
}

// YAGPDB event
func HandleUpdateStreaming(event *pubsub.Event) {
	logger.Info("Received update streaming event ", event.TargetGuild)

	gs := bot.State.GetGuild(event.TargetGuildInt)
	if gs == nil {
		return
	}

	cachedConfig.Delete(event.TargetGuildInt)
	CheckGuildFull(gs, true)
}

func CheckGuildFull(gs *dstate.GuildSet, fetchMembers bool) {

	config, err := GetConfig(gs.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).Error("Failed retrieving streaming config")
		return
	}

	if !config.Enabled {
		return
	}

	var wg sync.WaitGroup

	slowCheck := make([]*dstate.MemberState, 0)

	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(gs.ID), func(conn radix.Conn) error {

		bot.State.IterateMembers(gs.ID, func(chunk []*dstate.MemberState) bool {
			for _, ms := range chunk {
				if ms.Member == nil || ms.Presence == nil {

					if ms.Presence != nil && fetchMembers {
						// If were fetching members, then fetch the missing members
						// TODO: Maybe use the gateway request for this?
						slowCheck = append(slowCheck, ms)
						wg.Add(1)
						go func(gID, uID int64) {
							bot.GetMember(gID, uID)
							wg.Done()

						}(gs.ID, ms.User.ID)
					}

					continue
				}

				err = CheckPresence(conn, config, ms, gs)

				if err != nil {
					logger.WithError(err).Error("Error checking presence")
					continue
				}
			}

			return true
		})

		return nil
	}))

	if fetchMembers {
		wg.Wait()
	} else {
		return
	}

	logger.WithField("guild", gs.ID).Info("Starting slowcheck")

	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(gs.ID), func(conn radix.Conn) error {
		for _, ms := range slowCheck {

			if ms.Member == nil || ms.Presence == nil {
				continue
			}

			err = CheckPresence(conn, config, ms, gs)
			if err != nil {
				logger.WithError(err).Error("Error checking presence")
				continue
			}
		}

		return nil
	}))

	logger.WithField("guild", gs.ID).Info("Done slowcheck")
}

func HandleGuildMemberUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	m := evt.GuildMemberUpdate()

	if !evt.HasFeatureFlag(featureFlagEnabled) {
		return false, nil
	}

	config, err := BotCachedGetConfig(evt.GS.ID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.Enabled {
		return false, nil
	}

	ms := bot.State.GetMember(m.GuildID, m.User.ID)
	if ms == nil {
		logger.WithField("guild", m.GuildID).Error("Member not found in state")
		return false, nil
	}

	if ms.User.Bot {
		logger.WithField("isBot", m.User.Bot).Info("Ignoring Bots")
		return false, nil
	}

	if ms.Presence == nil {
		return // no presence tracked, no poing in continuing
	}

	err = CheckPresence(common.RedisPool, config, ms, evt.GS)
	if err != nil {
		return bot.CheckDiscordErrRetry(err), errors.WithStackIf(err)
	}

	return false, nil
}

func HandleGuildCreate(evt *eventsystem.EventData) {

	g := evt.GuildCreate()

	if !evt.HasFeatureFlag(featureFlagEnabled) {
		return
	}

	config, err := GetConfig(g.ID)
	if err != nil {
		logger.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	if !config.Enabled {
		return
	}
	gs := bot.State.GetGuild(g.ID)
	if gs == nil {
		logger.WithField("guild", g.ID).Error("Guild not found in state")
		return
	}

	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(g.ID), func(conn radix.Conn) error {

		bot.State.IterateMembers(g.ID, func(chunk []*dstate.MemberState) bool {
			for _, ms := range chunk {

				if ms.Member == nil || ms.Presence == nil {
					continue
				}

				err = CheckPresence(conn, config, ms, gs)
				if err != nil {
					logger.WithError(err).Error("Failed checking presence")
				}

			}

			return true
		})

		return nil
	}))
}

func HandlePresenceUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	p := evt.PresenceUpdate()

	gs := evt.GS

	if !evt.HasFeatureFlag(featureFlagEnabled) {
		return false, nil
	}

	config, err := BotCachedGetConfig(gs.ID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.Enabled || (config.GiveRole == 0 && (config.AnnounceMessage == "" || gs.GetChannel(config.AnnounceChannel) == nil)) {
		// Don't bother trying to send anything, its not "fully" enabled
		return
	}

	err = CheckPresenceSparse(common.RedisPool, config, &p.Presence, gs)
	if err != nil {
		return bot.CheckDiscordErrRetry(err), errors.WrapIff(err, "failed checking presence for %d", p.User.ID)
	}

	return false, nil
}

func CheckPresenceSparse(client radix.Client, config *Config, p *discordgo.Presence, gs *dstate.GuildSet) error {
	if !config.Enabled {
		return nil
	}

	mainActivity := retrieveMainActivity(p)
	ms, err := bot.GetMember(gs.ID, p.User.ID)
	if err != nil {
		return err
	}

	// Now the real fun starts
	// Either add or remove the stream
	if p.Status != discordgo.StatusOffline && mainActivity != nil && mainActivity.URL != "" && mainActivity.Type == 1 && !ms.User.Bot {

		// Streaming and not a bot
		if !config.MeetsRequirements(ms.Member.Roles, mainActivity.State, mainActivity.Details) {
			RemoveStreaming(client, config, gs.ID, p.User.ID, ms.Member.Roles)
			return nil
		}

		if config.GiveRole != 0 {
			go GiveStreamingRole(gs.ID, p.User.ID, config.GiveRole, ms.Member.Roles)
		}

		// if true, then we were marked now, and not before
		var markedNow bool
		client.Do(radix.FlatCmd(&markedNow, "SADD", KeyCurrentlyStreaming(gs.ID), p.User.ID))
		if !markedNow {
			// Already marked
			return nil
		}

		// Send the streaming announcement if enabled
		if config.AnnounceChannel != 0 && config.AnnounceMessage != "" {
			if err != nil {
				return errors.WithStackIf(err)
			}
			go SendStreamingAnnouncement(config, gs, ms, mainActivity.URL, mainActivity.State, mainActivity.Details, mainActivity.Name)
		}
	} else {
		// Not streaming
		RemoveStreamingSparse(client, config, gs.ID, p.User.ID)
	}

	return nil
}

func retrieveMainActivity(p *discordgo.Presence) *discordgo.Game {
	for _, v := range p.Activities {
		if v.Type == discordgo.GameTypeStreaming {
			return v
		}
	}

	if len(p.Activities) > 0 {
		return p.Activities[0]
	}

	return nil
}

func CheckPresence(client radix.Client, config *Config, ms *dstate.MemberState, gs *dstate.GuildSet) error {
	if !config.Enabled {
		return nil
	}

	// Now the real fun starts
	// Either add or remove the stream
	if ms.Presence != nil && ms.Presence.Status != dstate.StatusOffline && ms.Presence.Game != nil && ms.Presence.Game.URL != "" && ms.Presence.Game.Type == 1 && !ms.User.Bot {
		// Streaming and not a bot

		if !config.MeetsRequirements(ms.Member.Roles, ms.Presence.Game.State, ms.Presence.Game.Details) {
			RemoveStreaming(client, config, gs.ID, ms.User.ID, ms.Member.Roles)
			return nil
		}

		if config.GiveRole != 0 {
			go GiveStreamingRole(gs.ID, ms.User.ID, config.GiveRole, ms.Member.Roles)
		}

		// if true, then we were marked now, and not before
		var markedNow bool
		client.Do(radix.FlatCmd(&markedNow, "SADD", KeyCurrentlyStreaming(gs.ID), ms.User.ID))
		if !markedNow {
			// Already marked
			return nil
		}

		// Send the streaming announcement if enabled
		if config.AnnounceChannel != 0 && config.AnnounceMessage != "" {
			go SendStreamingAnnouncement(config, gs, ms, ms.Presence.Game.URL, ms.Presence.Game.State, ms.Presence.Game.Details, ms.Presence.Game.Name)
		}

	} else {
		// Not streaming
		RemoveStreaming(client, config, gs.ID, ms.User.ID, ms.Member.Roles)
	}

	return nil
}

func (config *Config) MeetsRequirements(roles []int64, activityState, activityDetails string) bool {
	// Check if they have the required role
	if config.RequireRole != 0 {
		if !common.ContainsInt64Slice(roles, config.RequireRole) {
			// Dosen't have required role
			return false
		}
	}

	// Check if they have a ignored role
	if config.IgnoreRole != 0 {
		if common.ContainsInt64Slice(roles, config.IgnoreRole) {
			// We ignore people with this role.. :'(
			return false
		}
	}

	if strings.TrimSpace(config.GameRegex) != "" {
		gameName := activityState
		compiledRegex, err := regexp.Compile(strings.TrimSpace(config.GameRegex))
		if err == nil {
			// It should be verified before this that its valid
			if !compiledRegex.MatchString(gameName) {
				return false
			}
		}
	}

	if strings.TrimSpace(config.TitleRegex) != "" {
		streamTitle := activityDetails
		compiledRegex, err := regexp.Compile(strings.TrimSpace(config.TitleRegex))
		if err == nil {
			// It should be verified before this that its valid
			if !compiledRegex.MatchString(streamTitle) {
				return false
			}
		}
	}

	return true
}

func RemoveStreamingSparse(client radix.Client, config *Config, guildID int64, memberID int64) {
	var removed bool
	client.Do(radix.FlatCmd(&removed, "SREM", KeyCurrentlyStreaming(guildID), memberID))

	if removed && config.GiveRole != 0 {
		common.BotSession.GuildMemberRoleRemove(guildID, memberID, config.GiveRole)
	}
}

func RemoveStreaming(client radix.Client, config *Config, guildID int64, memberID int64, currentRoles []int64) {
	client.Do(radix.FlatCmd(nil, "SREM", KeyCurrentlyStreaming(guildID), memberID))
	go RemoveStreamingRole(guildID, memberID, config.GiveRole, currentRoles)

	// Was not streaming before if we removed 0 elements
	// var removed bool
	// client.Do(radix.FlatCmd(&removed, "SREM", KeyCurrentlyStreaming(guildID), memberID))
	// if removed && config.GiveRole != 0 {
	// 	go common.BotSession.GuildMemberRoleRemove(guildID, memberID, config.GiveRole)
	// }
}

func SendStreamingAnnouncement(config *Config, guild *dstate.GuildSet, ms *dstate.MemberState, url string, gameName string, streamTitle string, streamPlatform string) {
	// Only send one announcment every 1 hour
	var resp string
	key := fmt.Sprintf("streaming_announcement_sent:%d:%d", guild.ID, ms.User.ID)
	err := common.RedisPool.Do(radix.Cmd(&resp, "SET", key, "1", "EX", "3600", "NX"))
	if err != nil {
		logger.WithError(err).Error("failed setting streaming announcment cooldown")
		return
	}

	if resp != "OK" {
		logger.Info("streaming announcment cooldown: ", ms.User.ID)
		return
	}

	// make sure the channel exists
	foundChannel := false
	for _, v := range guild.Channels {
		if v.ID == config.AnnounceChannel {
			foundChannel = true
		}
	}

	// unknown channel, disable announcements
	if !foundChannel {
		config.AnnounceChannel = 0
		config.Save(guild.ID)

		logger.WithField("guild", guild.ID).WithField("channel", config.AnnounceChannel).Warn("Channel not found in state, not sending streaming announcement")
		return
	}

	go analytics.RecordActiveUnit(guild.ID, &Plugin{}, "sent_streaming_announcement")

	ctx := templates.NewContext(guild, nil, ms)
	// ctx.Data["URL"] = ms.PresenceGame.URL
	// ctx.Data["url"] = ms.PresenceGame.URL
	// ctx.Data["Game"] = ms.PresenceGame.State
	// ctx.Data["StreamTitle"] = ms.PresenceGame.Details
	// ctx.Data["StreamPlatform"] = ms.PresenceGame.Name

	ctx.Data["URL"] = url
	ctx.Data["url"] = url
	ctx.Data["Game"] = gameName
	ctx.Data["StreamTitle"] = streamTitle
	ctx.Data["StreamPlatform"] = streamPlatform

	out, err := ctx.Execute(config.AnnounceMessage)
	if err != nil {
		logger.WithError(err).WithField("guild", guild.ID).Warn("Failed executing template")
		return
	}

	m, err := common.BotSession.ChannelMessageSendComplex(config.AnnounceChannel, ctx.MessageSend(out))
	if err == nil && ctx.CurrentFrame.DelResponse {
		templates.MaybeScheduledDeleteMessage(guild.ID, config.AnnounceChannel, m.ID, ctx.CurrentFrame.DelResponseDelay)
	}
}

func GiveStreamingRole(guildID, memberID, streamingRole int64, currentUserRoles []int64) {
	if streamingRole == 0 {
		return
	}

	var err error

	if !common.ContainsInt64Slice(currentUserRoles, streamingRole) {
		err = common.BotSession.GuildMemberRoleAdd(guildID, memberID, streamingRole)
		go analytics.RecordActiveUnit(guildID, &Plugin{}, "assigned_streaming_role")

	}

	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingAccess) {
			DisableStreamingRole(guildID)
		}

		logger.WithError(err).WithField("guild", guildID).WithField("user", memberID).Error("Failed adding streaming role")
		common.RedisPool.Do(radix.FlatCmd(nil, "SREM", KeyCurrentlyStreaming(guildID), memberID))
	}
}

func RemoveStreamingRole(guildID, memberID int64, streamingRole int64, currentRoles []int64) {
	if streamingRole == 0 {
		return
	}

	if !common.ContainsInt64Slice(currentRoles, streamingRole) {
		return
	}

	go analytics.RecordActiveUnit(guildID, &Plugin{}, "removed_streaming_role")

	err := common.BotSession.GuildMemberRoleRemove(guildID, memberID, streamingRole)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("user", memberID).WithField("role", streamingRole).Error("Failed removing streaming role")
		if common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingAccess) {
			DisableStreamingRole(guildID)
		}
	}
}

func DisableStreamingRole(guildID int64) {
	logger.WithField("guild", guildID).Warn("Disabling streaming role for server because of misssing permissions or unknown role")

	conf, err := GetConfig(guildID)
	if err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("Failed retrieving streaming config, when there should be one?")
		return
	}

	conf.GiveRole = 0
	conf.Save(guildID)
	featureflags.MarkGuildDirty(guildID)
}

var cachedConfig = common.CacheSet.RegisterSlot("streaming_configs", nil, int64(0))

func BotCachedGetConfig(guildID int64) (*Config, error) {
	v, err := cachedConfig.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		return GetConfig(guildID)
	})

	if err != nil {
		return nil, err
	}

	return v.(*Config), nil
}
