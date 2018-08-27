package streaming

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/mediocregopher/radix.v3"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"sync"
)

func KeyCurrentlyStreaming(gID int64) string { return "currently_streaming:" + discordgo.StrID(gID) }

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.ShardMigrationHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(HandlePresenceUpdate, eventsystem.EventPresenceUpdate)
	eventsystem.AddHandler(HandleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)
	pubsub.AddHandler("update_streaming", HandleUpdateStreaming, nil)
}

func (p *Plugin) GuildMigrated(gs *dstate.GuildState, toThisSlave bool) {
	if !toThisSlave {
		return
	}

	go CheckGuildFull(gs, false)
}

// YAGPDB event
func HandleUpdateStreaming(event *pubsub.Event) {
	log.Info("Received update streaming event ", event.TargetGuild)

	gs := bot.State.Guild(true, event.TargetGuildInt)
	if gs == nil {
		return
	}

	CheckGuildFull(gs, true)
}

func CheckGuildFull(gs *dstate.GuildState, fetchMembers bool) {

	config, err := GetConfig(gs.ID)
	if err != nil {
		log.WithError(err).WithField("guild", gs.ID).Error("Failed retrieving streaming config")
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

			err = CheckPresence(conn, config, ms, gs)
			if err != nil {
				log.WithError(err).Error("Error checking presence")
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

	log.WithField("guild", gs.ID).Info("Starting slowcheck")

	gs.RLock()
	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(gs.ID), func(conn radix.Conn) error {
		for _, ms := range slowCheck {

			if !ms.MemberSet || !ms.PresenceSet {
				continue
			}

			err = CheckPresence(conn, config, ms, gs)
			if err != nil {
				log.WithError(err).Error("Error checking presence")
				continue
			}
		}

		return nil
	}))
	gs.RUnlock()

	log.WithField("guild", gs.ID).Info("Done slowcheck")
}

func HandleGuildMemberUpdate(evt *eventsystem.EventData) {
	m := evt.GuildMemberUpdate()

	config, err := GetConfig(m.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	if !config.Enabled {
		return
	}

	gs := bot.State.Guild(true, m.GuildID)
	if gs == nil {
		log.WithField("guild", m.GuildID).Error("Guild not found in state")
		return
	}

	ms := gs.Member(true, m.User.ID)
	if ms == nil {
		log.WithField("guild", m.GuildID).Error("Member not found in state")
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	if !ms.PresenceSet {
		log.WithField("guild", m.GuildID).Warn("Presence not found in state")
		return
	}

	err = CheckPresence(common.RedisPool, config, ms, gs)
	if err != nil {
		log.WithError(err).Error("Failed checking presence")
	}
}

func HandleGuildCreate(evt *eventsystem.EventData) {

	g := evt.GuildCreate()

	config, err := GetConfig(g.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	if !config.Enabled {
		return
	}

	gs := bot.State.Guild(true, g.ID)
	if gs == nil {
		log.WithField("guild", g.ID).Error("Guild not found in state")
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	err = common.RedisPool.Do(radix.WithConn(KeyCurrentlyStreaming(g.ID), func(conn radix.Conn) error {

		for _, ms := range gs.Members {

			if !ms.MemberSet || !ms.PresenceSet {
				continue
			}

			err = CheckPresence(conn, config, ms, gs)

			if err != nil {
				log.WithError(err).Error("Failed checking presence")
			}
		}

		return nil
	}))
}

func HandlePresenceUpdate(evt *eventsystem.EventData) {
	p := evt.PresenceUpdate()

	config, err := GetConfig(p.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	if !config.Enabled {
		return
	}

	gs := bot.State.Guild(true, p.GuildID)
	if gs == nil {
		log.WithField("guild", p.GuildID).Error("Failed retrieving guild from state")
		return
	}

	ms, err := bot.GetMember(p.GuildID, p.User.ID)
	if ms == nil || err != nil {
		log.WithError(err).WithField("guild", p.GuildID).WithField("user", p.User.ID).Error("Failed retrieving member")
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	err = CheckPresence(common.RedisPool, config, ms, gs)
	if err != nil {
		log.WithError(err).WithField("guild", p.GuildID).Error("Failed checking presence")
	}
}

func CheckPresence(client radix.Client, config *Config, ms *dstate.MemberState, gs *dstate.GuildState) error {
	if !config.Enabled {
		// RemoveStreaming(client, config, gs.ID, p.User.ID, member)
		return nil
	}

	// Now the real fun starts
	// Either add or remove the stream
	if ms.PresenceStatus != dstate.StatusOffline && ms.PresenceGame != nil && ms.PresenceGame.URL != "" {
		// Streaming

		if !config.MeetsRequirements(ms) {
			RemoveStreaming(client, config, gs.ID, ms)
			return nil
		}

		if config.GiveRole != 0 {
			go GiveStreamingRole(ms, config.GiveRole, gs.Guild)
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
		RemoveStreaming(client, config, gs.ID, ms)
	}

	return nil
}

func (config *Config) MeetsRequirements(ms *dstate.MemberState) bool {
	// Check if they have the required role
	if config.RequireRole != 0 {
		if !common.ContainsInt64Slice(ms.Roles, config.RequireRole) {
			// Dosen't have required role
			return false
		}
	}

	// Check if they have a ignored role
	if config.IgnoreRole != 0 {
		if common.ContainsInt64Slice(ms.Roles, config.IgnoreRole) {
			// We ignore people with this role.. :'(
			return false
		}
	}

	if strings.TrimSpace(config.GameRegex) != "" {
		gameName := ms.PresenceGame.Details
		compiledRegex, err := regexp.Compile(strings.TrimSpace(config.GameRegex))
		if err == nil {
			// It should be verified before this that its valid
			if !compiledRegex.MatchString(gameName) {
				return false
			}
		}
	}

	if strings.TrimSpace(config.TitleRegex) != "" {
		streamTitle := ms.PresenceGame.Name
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

func RemoveStreaming(client radix.Client, config *Config, guildID int64, ms *dstate.MemberState) {
	if ms.MemberSet {
		client.Do(radix.FlatCmd(nil, "SREM", KeyCurrentlyStreaming(guildID), ms.ID))
		go RemoveStreamingRole(ms, config.GiveRole, guildID)
	} else {
		// Was not streaming before if we removed 0 elements
		var removed bool
		client.Do(radix.FlatCmd(&removed, "SREM", KeyCurrentlyStreaming(guildID), ms.ID))
		if removed && config.GiveRole != 0 {
			go common.BotSession.GuildMemberRoleRemove(guildID, ms.ID, config.GiveRole)
		}
	}
}

func SendStreamingAnnouncement(config *Config, guild *dstate.GuildState, ms *dstate.MemberState) {
	foundChannel := false
	for _, v := range guild.Channels {
		if v.ID == config.AnnounceChannel {
			foundChannel = true
		}
	}

	if !foundChannel {
		log.WithField("guild", guild.ID).WithField("channel", config.AnnounceChannel).Warn("Channel not found in state, not sending streaming announcement")
		return
	}

	ctx := templates.NewContext(guild, nil, ms)
	ctx.Data["URL"] = common.EscapeSpecialMentions(ms.PresenceGame.URL)
	ctx.Data["url"] = common.EscapeSpecialMentions(ms.PresenceGame.URL)
	ctx.Data["Game"] = ms.PresenceGame.Details
	ctx.Data["StreamTitle"] = ms.PresenceGame.Name

	guild.RUnlock()
	out, err := ctx.Execute(config.AnnounceMessage)
	guild.RLock()
	if err != nil {
		log.WithError(err).WithField("guild", guild.ID).Warn("Failed executing template")
		return
	}

	common.BotSession.ChannelMessageSend(config.AnnounceChannel, out)
}

func GiveStreamingRole(ms *dstate.MemberState, role int64, guild *discordgo.Guild) {
	if role == 0 {
		return
	}

	err := common.AddRoleDS(ms, role)

	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingAccess) {
			DisableStreamingRole(guild.ID)
		}

		log.WithError(err).WithField("guild", guild.ID).WithField("user", ms.ID).Error("Failed adding streaming role")
		common.RedisPool.Do(radix.FlatCmd(nil, "SREM", KeyCurrentlyStreaming(guild.ID), ms.ID))
	}
}

func RemoveStreamingRole(ms *dstate.MemberState, role int64, guildID int64) {
	if role == 0 {
		return
	}

	err := common.RemoveRoleDS(ms, role)
	if err != nil {
		log.WithError(err).WithField("guild", guildID).WithField("user", ms.ID).WithField("role", role).Error("Failed removing streaming role")
		if common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingAccess) {
			DisableStreamingRole(guildID)
		}
	}
}

func DisableStreamingRole(guildID int64) {
	log.WithField("guild", guildID).Warn("Disabling streaming role for server because of misssing permissions or unknown role")

	conf, err := GetConfig(guildID)
	if err != nil {
		log.WithField("guild", guildID).WithError(err).Error("Failed retrieving streaming config, when there should be one?")
		return
	}

	conf.GiveRole = 0
	conf.Save(guildID)
}
