package streaming

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"emperror.dev/errors"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/mediocregopher/radix/v3"
)

func KeyCurrentlyStreaming(gID int64) string { return "currently_streaming:" + discordgo.StrID(gID) }

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLast(p, HandlePresenceUpdate, eventsystem.EventPresenceUpdate)
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)
	pubsub.AddHandler("update_streaming", HandleUpdateStreaming, nil)
}

// YAGPDB event
func HandleUpdateStreaming(event *pubsub.Event) {
	logger.Info("Received update streaming event ", event.TargetGuild)

	gs := bot.State.Guild(true, event.TargetGuildInt)
	if gs == nil {
		return
	}

	gs.UserCacheDel(CacheKeyConfig)

	CheckGuildFull(gs, true)
}

func CheckGuildFull(gs *dstate.GuildState, fetchMembers bool) {

	config, err := GetConfig(gs.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).Error("Failed retrieving streaming config")
		return
	}

	if !config.Enabled {
		return
	}

	gs.RLock()

	var wg sync.WaitGroup

	slowCheck := make([]*dstate.MemberState, 0, len(gs.Members))

	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(gs.ID), func(conn radix.Conn) error {
		for _, ms := range gs.Members {

			if !ms.MemberSet || !ms.PresenceSet {

				if ms.PresenceSet && fetchMembers {
					// If were fetching members, then fetch the missing members
					// TODO: Maybe use the gateway request for this?
					slowCheck = append(slowCheck, ms)
					wg.Add(1)
					go func(gID, uID int64) {
						bot.GetMember(gID, uID)
						wg.Done()

					}(gs.ID, ms.ID)
				}

				continue
			}

			gs.RUnlock()
			err = CheckPresence(conn, config, ms, gs)
			gs.RLock()

			if err != nil {
				logger.WithError(err).Error("Error checking presence")
				continue
			}
		}

		return nil
	}))

	gs.RUnlock()

	if fetchMembers {
		wg.Wait()
	} else {
		return
	}

	logger.WithField("guild", gs.ID).Info("Starting slowcheck")

	gs.RLock()
	defer gs.RUnlock()
	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(gs.ID), func(conn radix.Conn) error {
		for _, ms := range slowCheck {

			if !ms.MemberSet || !ms.PresenceSet {
				continue
			}

			gs.RUnlock()
			err = CheckPresence(conn, config, ms, gs)
			gs.RLock()
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

	config, err := BotCachedGetConfig(evt.GS)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.Enabled {
		return false, nil
	}

	ms := evt.GS.MemberCopy(true, m.User.ID)
	if ms == nil {
		logger.WithField("guild", m.GuildID).Error("Member not found in state")
		return false, nil
	}

	if !ms.PresenceSet {
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
	gs := bot.State.Guild(true, g.ID)
	if gs == nil {
		logger.WithField("guild", g.ID).Error("Guild not found in state")
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(g.ID), func(conn radix.Conn) error {

		for _, ms := range gs.Members {

			if !ms.MemberSet || !ms.PresenceSet {
				continue
			}

			gs.RUnlock()
			err = CheckPresence(conn, config, ms, gs)
			gs.RLock()

			if err != nil {
				logger.WithError(err).Error("Failed checking presence")
			}
		}

		return nil
	}))
}

func HandlePresenceUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	p := evt.PresenceUpdate()

	gs := evt.GS

	if !evt.HasFeatureFlag(featureFlagEnabled) {
		return false, nil
	}

	config, err := BotCachedGetConfig(gs)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.Enabled || (config.GiveRole == 0 && (config.AnnounceMessage == "" || gs.Channel(true, config.AnnounceChannel) == nil)) {
		// Don't bother trying to send anything, its not "fully" enabled
		return
	}

	err = CheckPresenceSparse(common.RedisPool, config, &p.Presence, gs)
	if err != nil {
		return bot.CheckDiscordErrRetry(err), errors.WrapIff(err, "failed checking presence for %d", p.User.ID)
	}

	return false, nil
}

func CheckPresenceSparse(client radix.Client, config *Config, p *discordgo.Presence, gs *dstate.GuildState) error {
	if !config.Enabled {
		// RemoveStreaming(client, config, gs.ID, p.User.ID, member)
		return nil
	}

	mainActivity := retrieveMainActivity(p)

	// Now the real fun starts
	// Either add or remove the stream
	if p.Status != discordgo.StatusOffline && mainActivity != nil && mainActivity.URL != "" && mainActivity.Type == 1 {

		// Streaming

		if !config.MeetsRequirements(p.Roles, mainActivity.State, mainActivity.Details) {
			RemoveStreaming(client, config, gs.ID, p.User.ID, p.Roles)
			return nil
		}

		if config.GiveRole != 0 {
			go GiveStreamingRole(gs.ID, p.User.ID, config.GiveRole, p.Roles)
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
			ms, err := bot.GetMember(gs.ID, p.User.ID)
			if err != nil {
				return errors.WithStackIf(err)
			}

			SendStreamingAnnouncement(config, gs, ms)
		}

	} else {
		// Not streaming
		RemoveStreaming(client, config, gs.ID, p.User.ID, p.Roles)
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

func CheckPresence(client radix.Client, config *Config, ms *dstate.MemberState, gs *dstate.GuildState) error {
	if !config.Enabled {
		// RemoveStreaming(client, config, gs.ID, p.User.ID, member)
		return nil
	}

	// Now the real fun starts
	// Either add or remove the stream
	if ms.PresenceStatus != dstate.StatusOffline && ms.PresenceGame != nil && ms.PresenceGame.URL != "" && ms.PresenceGame.Type == 1 {
		// Streaming

		if !config.MeetsRequirements(ms.Roles, ms.PresenceGame.State, ms.PresenceGame.Details) {
			RemoveStreaming(client, config, gs.ID, ms.ID, ms.Roles)
			return nil
		}

		if config.GiveRole != 0 {
			go GiveStreamingRole(gs.Guild.ID, ms.ID, config.GiveRole, ms.Roles)
		}

		// if true, then we were marked now, and not before
		var markedNow bool
		client.Do(radix.FlatCmd(&markedNow, "SADD", KeyCurrentlyStreaming(gs.ID), ms.ID))
		if !markedNow {
			// Already marked
			return nil
		}

		// Send the streaming announcement if enabled
		if config.AnnounceChannel != 0 && config.AnnounceMessage != "" {
			SendStreamingAnnouncement(config, gs, ms)
		}

	} else {
		// Not streaming
		RemoveStreaming(client, config, gs.ID, ms.ID, ms.Roles)
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

func SendStreamingAnnouncement(config *Config, guild *dstate.GuildState, ms *dstate.MemberState) {
	// Only send one announcment every 1 hour
	var resp string
	key := fmt.Sprintf("streaming_announcement_sent:%d:%d", guild.ID, ms.ID)
	err := common.RedisPool.Do(radix.Cmd(&resp, "SET", key, "1", "EX", "3600", "NX"))
	if err != nil {
		logger.WithError(err).Error("failed setting streaming announcment cooldown")
		return
	}

	if resp != "OK" {
		logger.Info("streaming announcment cooldown: ", ms.ID)
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
	ctx.Data["URL"] = ms.PresenceGame.URL
	ctx.Data["url"] = ms.PresenceGame.URL
	ctx.Data["Game"] = ms.PresenceGame.State
	ctx.Data["StreamTitle"] = ms.PresenceGame.Details
	ctx.Data["StreamPlatform"] = ms.PresenceGame.Name

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

type CacheKey int

const (
	CacheKeyConfig CacheKey = iota
)

func BotCachedGetConfig(gs *dstate.GuildState) (*Config, error) {
	v, err := gs.UserCacheFetch(CacheKeyConfig, func() (interface{}, error) {
		return GetConfig(gs.ID)
	})

	if err != nil {
		return nil, err
	}

	return v.(*Config), nil
}
