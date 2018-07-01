package streaming

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/mediocregopher/radix.v2/redis"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"sync"
)

func KeyCurrentlyStreaming(gID int64) string { return "currently_streaming:" + discordgo.StrID(gID) }

func (p *Plugin) InitBot() {

	eventsystem.AddHandler(bot.ConcurrentEventHandler(bot.RedisWrapper(HandleGuildCreate)), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(bot.RedisWrapper(HandlePresenceUpdate), eventsystem.EventPresenceUpdate)
	eventsystem.AddHandler(bot.RedisWrapper(HandleGuildMemberUpdate), eventsystem.EventGuildMemberUpdate)

}

var _ bot.BotStarterHandler = (*Plugin)(nil)

func (p *Plugin) StartBot() {
	pubsub.AddHandler("update_streaming", HandleUpdateStreaming, nil)
}

// YAGPDB event
func HandleUpdateStreaming(event *pubsub.Event) {
	log.Info("Received the streaming event boi ", event.TargetGuild)

	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection from pool")
		return
	}
	defer common.RedisPool.Put(client)

	gs := bot.State.Guild(true, event.TargetGuildInt)
	if gs == nil {
		log.WithField("guild", event.TargetGuild).Error("Guild not found in state")
		return
	}

	config, err := GetConfig(client, gs.ID())
	if err != nil {
		log.WithError(err).WithField("guild", gs.ID()).Error("Failed retrieving streaming config")
	}

	gs.RLock()

	var wg sync.WaitGroup

	slowCheck := make([]*dstate.MemberState, 0, len(gs.Members))
	for _, ms := range gs.Members {

		if ms.Member == nil || ms.Presence == nil {
			if ms.Presence != nil {
				slowCheck = append(slowCheck, ms)
				wg.Add(1)
				go func(gID, uID int64) {
					bot.GetMember(gID, uID)
					wg.Done()
				}(gs.ID(), ms.ID())
			}
			continue
		}

		err = CheckPresence(client, config, ms.Presence, ms.Member, gs)
		if err != nil {
			log.WithError(err).Error("Error checking presence")
			continue
		}
	}

	gs.RUnlock()

	wg.Wait()

	log.WithField("guild", gs.ID()).Info("Starting slowcheck")
	gs.RLock()
	for _, ms := range slowCheck {

		if ms.Member == nil || ms.Presence == nil {
			continue
		}

		err = CheckPresence(client, config, ms.Presence, ms.Member, gs)
		if err != nil {
			log.WithError(err).Error("Error checking presence")
			continue
		}
	}
	gs.RUnlock()
	log.WithField("guild", gs.ID()).Info("Done slowcheck")
}

func HandleGuildMemberUpdate(evt *eventsystem.EventData) {
	client := bot.ContextRedis(evt.Context())
	m := evt.GuildMemberUpdate()

	config, err := GetConfig(client, m.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
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

	if ms.Presence == nil {
		log.WithField("guild", m.GuildID).Warn("Presence not found in state")
		return
	}

	err = CheckPresence(client, config, ms.Presence, m.Member, gs)
	if err != nil {
		log.WithError(err).Error("Failed checking presence")
	}
}

func HandleGuildCreate(evt *eventsystem.EventData) {

	client := bot.ContextRedis(evt.Context())
	g := evt.GuildCreate()

	config, err := GetConfig(client, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	gs := bot.State.Guild(true, g.ID)
	if gs == nil {
		log.WithField("guild", g.ID).Error("Guild not found in state")
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	for _, p := range g.Presences {

		var member *discordgo.Member

		for _, v := range g.Members {
			if v.User.ID == p.User.ID {
				member = v
			}
		}

		if member == nil {
			log.Error("No member found")
			continue
		}

		err = CheckPresence(client, config, p, member, gs)

		if err != nil {
			log.WithError(err).Error("Failed checking presence")
		}
	}
}

func HandlePresenceUpdate(evt *eventsystem.EventData) {
	client := bot.ContextRedis(evt.Context())
	p := evt.PresenceUpdate()

	config, err := GetConfig(client, p.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	gs := bot.State.Guild(true, p.GuildID)
	if gs == nil {
		log.WithField("guild", p.GuildID).Error("Failed retrieving guild from state")
		return
	}

	// member, _ := bot.GetMember(gs.ID(), p.User.ID)

	gs.RLock()
	defer gs.RUnlock()

	ms := gs.Member(false, p.User.ID)

	var member *discordgo.Member
	if ms != nil {
		member = ms.Member
	}

	err = CheckPresence(client, config, &p.Presence, member, gs)
	if err != nil {
		log.WithError(err).WithField("guild", p.GuildID).Error("Failed checking presence")
	}
}

func CheckPresence(client *redis.Client, config *Config, p *discordgo.Presence, member *discordgo.Member, gs *dstate.GuildState) error {
	if !config.Enabled {
		// RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
		return nil
	}

	// Now the real fun starts
	// Either add or remove the stream
	if p.Status != "offline" && p.Game != nil && p.Game.URL != "" {
		// Streaming

		if member == nil {
			// Member is required from on here
			var err error
			gs.RUnlock()
			member, err = bot.GetMember(gs.ID(), p.User.ID)
			gs.RLock()
			if err != nil {
				return err
			}
		}

		if !config.MeetsRequirements(member, p) {
			RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
			return nil
		}

		if config.GiveRole != 0 {
			err := GiveStreamingRole(member, config.GiveRole, gs.Guild)
			if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownRole) {
				log.WithError(err).WithField("guild", gs.ID()).WithField("user", member.User.ID).Error("Failed adding streaming role")
				client.Cmd("SREM", KeyCurrentlyStreaming(gs.ID()), member.User.ID)
			}
		}

		// Was already marked as streaming before if we added 0 elements
		if num, _ := client.Cmd("SADD", KeyCurrentlyStreaming(gs.ID()), member.User.ID).Int(); num == 0 {
			return nil
		}

		// Send the streaming announcement if enabled
		if config.AnnounceChannel != 0 && config.AnnounceMessage != "" {
			SendStreamingAnnouncement(client, config, gs, member, p)
		}

	} else {
		// Not streaming
		RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
	}

	return nil
}

func (config *Config) MeetsRequirements(member *discordgo.Member, p *discordgo.Presence) bool {
	// Check if they have the required role
	if config.RequireRole != 0 {
		found := false
		for _, role := range member.Roles {
			if role == config.RequireRole {
				found = true
				break
			}
		}

		// Dosen't have atleast one required role
		if !found {
			return false
		}
	}

	// Check if they have a ignored role
	if config.IgnoreRole != 0 {
		for _, role := range member.Roles {
			// We ignore people with this role.. :')
			if role == config.IgnoreRole {
				return false
			}
		}
	}

	if strings.TrimSpace(config.GameRegex) != "" {
		gameName := p.Game.Details
		compiledRegex, err := regexp.Compile(strings.TrimSpace(config.GameRegex))
		if err == nil {
			// It should be verified before this that its valid
			if !compiledRegex.MatchString(gameName) {
				return false
			}
		}
	}

	if strings.TrimSpace(config.TitleRegex) != "" {
		streamTitle := p.Game.Name
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

func RemoveStreaming(client *redis.Client, config *Config, guildID int64, userID int64, member *discordgo.Member) {
	if member != nil {
		RemoveStreamingRole(member, config.GiveRole, guildID)
		client.Cmd("SREM", KeyCurrentlyStreaming(guildID), userID)
	} else {
		// Was not streaming before if we removed 0 elements
		if n, _ := client.Cmd("SREM", KeyCurrentlyStreaming(guildID), userID).Int(); n > 0 && config.GiveRole != 0 {
			common.BotSession.GuildMemberRoleRemove(guildID, userID, config.GiveRole)
		}
	}

}

func SendStreamingAnnouncement(client *redis.Client, config *Config, guild *dstate.GuildState, member *discordgo.Member, p *discordgo.Presence) {
	foundChannel := false
	for _, v := range guild.Channels {
		if v.ID() == config.AnnounceChannel {
			foundChannel = true
		}
	}

	if !foundChannel {
		log.WithField("guild", guild.ID()).WithField("channel", config.AnnounceChannel).Warn("Channel not found in state, not sending streaming announcement")
		return
	}

	ctx := templates.NewContext(bot.State.User(true).User, guild, nil, member)
	ctx.Data["URL"] = common.EscapeSpecialMentions(p.Game.URL)
	ctx.Data["url"] = common.EscapeSpecialMentions(p.Game.URL)
	ctx.Data["Game"] = p.Game.Details
	ctx.Data["StreamTitle"] = p.Game.Name

	guild.RUnlock()
	out, err := ctx.Execute(client, config.AnnounceMessage)
	guild.RLock()
	if err != nil {
		log.WithError(err).WithField("guild", guild.ID()).Warn("Failed executing template")
		return
	}

	common.BotSession.ChannelMessageSend(config.AnnounceChannel, out)
}

func GiveStreamingRole(member *discordgo.Member, role int64, guild *discordgo.Guild) error {
	// Ensure the role exists
	found := false
	for _, v := range guild.Roles {
		if v.ID == role {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	err := common.AddRole(member, role, guild.ID)
	return err
}

func RemoveStreamingRole(member *discordgo.Member, role int64, guildID int64) {
	if role == 0 {
		return
	}

	err := common.RemoveRole(member, role, guildID)
	if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownRole, discordgo.ErrCodeMissingAccess) {
		log.WithError(err).WithField("guild", guildID).WithField("user", member.User.ID).WithField("role", role).Error("Failed removing streaming role")
	}
}
