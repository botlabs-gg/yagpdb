package streaming

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/mediocregopher/radix.v2/redis"
	"sync"
)

func KeyCurrentlyStreaming(gID string) string { return "currently_streaming:" + gID }

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

	gs := bot.State.Guild(true, event.TargetGuild)
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
				go func(gID, uID string) {
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
	m := evt.GuildMemberUpdate
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
	g := evt.GuildCreate

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
	p := evt.PresenceUpdate
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

	// Now the real fun starts
	// Either add or remove the stream
	if p.Status != "offline" && p.Game != nil && p.Game.URL != "" {
		// Streaming

		// Only do these checks here to ensure we cleanup the user from the streaming set
		// even if the plugin was disabled or the user ended up on the ignored roles
		if !config.Enabled {
			RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
			return nil
		}

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

		if config.RequireRole != "" {
			found := false
			for _, role := range member.Roles {
				if role == config.RequireRole {
					found = true
					break
				}
			}

			// Dosen't the required role
			if !found {
				RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
				return nil
			}
		}

		if config.IgnoreRole != "" {
			for _, role := range member.Roles {
				// We ignore people with this role.. :')
				if role == config.IgnoreRole {
					RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
					return nil
				}
			}
		}

		// Was already marked as streaming before if we added 0 elements
		if num, _ := client.Cmd("SADD", KeyCurrentlyStreaming(gs.ID()), member.User.ID).Int(); num == 0 {
			return nil
		}

		// Send the streaming announcement if enabled
		if config.AnnounceChannel != "" && config.AnnounceMessage != "" {
			SendStreamingAnnouncement(client, config, gs, member, p)
		}

		if config.GiveRole != "" {
			err := GiveStreamingRole(member, config.GiveRole, gs.Guild)
			if err != nil {
				log.WithError(err).WithField("guild", gs.ID()).WithField("user", member.User.ID).Error("Failed adding streaming role")
				client.Cmd("SREM", KeyCurrentlyStreaming(gs.ID()), member.User.ID)
			}
		}

	} else {
		// Not streaming
		RemoveStreaming(client, config, gs.ID(), p.User.ID, member)
	}

	return nil
}

func RemoveStreaming(client *redis.Client, config *Config, guildID string, userID string, member *discordgo.Member) {
	// Was not streaming before if we removed 0 elements
	if num, _ := client.Cmd("SREM", KeyCurrentlyStreaming(guildID), userID).Int(); num == 0 {
		return
	}

	if member != nil {
		RemoveStreamingRole(member, config.GiveRole, guildID)
	} else {
		common.BotSession.GuildMemberRoleRemove(guildID, userID, config.GiveRole)
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
		log.WithField("guild", guild.ID()).WithField("channel", config.AnnounceChannel).Error("Channel not found in state")
		return
	}

	ctx := templates.NewContext(bot.State.User(true).User, guild, nil, member)
	ctx.Data["URL"] = common.EscapeSpecialMentions(p.Game.URL)
	ctx.Data["url"] = common.EscapeSpecialMentions(p.Game.URL)

	guild.RUnlock()
	out, err := ctx.Execute(client, config.AnnounceMessage)
	guild.RLock()
	if err != nil {
		log.WithError(err).WithField("guild", guild.ID()).Error("Failed executing template")
		return
	}

	common.BotSession.ChannelMessageSend(config.AnnounceChannel, out)
}

func GiveStreamingRole(member *discordgo.Member, role string, guild *discordgo.Guild) error {
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

func RemoveStreamingRole(member *discordgo.Member, role string, guildID string) {
	err := common.RemoveRole(member, role, guildID)
	if err != nil {
		log.WithError(err).WithField("guild", guildID).WithField("user", member.User.ID).Error("Failed removing streaming role")
	}
}
