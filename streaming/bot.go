package streaming

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
)

func (p *Plugin) InitBot() {
	common.BotSession.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(bot.CustomPresenceUpdate(HandlePresenceUpdate))
	common.BotSession.AddHandler(bot.CustomGuildMemberUpdate(HandleGuildMemberUpdate))

}

func (p *Plugin) StartBot() {
	pubsub.AddHandler("update_streaming", HandleUpdateStreaming, nil)
}

// YAGPDB event
func HandleUpdateStreaming(event *pubsub.Event) {
	log.Info("Received the streaming event boi", event.TargetGuild)

	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection from pool")
		return
	}

	guild, err := common.BotSession.State.Guild(event.TargetGuild)
	if err != nil {
		log.WithError(err).Error("Failed retrieving guild from state")
		return
	}

	config, err := GetConfig(client, guild.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
	}
	errs := 0
	for _, presence := range guild.Presences {

		var member *discordgo.Member

		for _, m := range guild.Members {
			if m.User.ID == presence.User.ID {
				member = m
				break
			}
		}

		if member == nil {
			log.Error("Member not found in guild", presence.User.ID)
			errs++
			continue
		}

		err = CheckPresence(client, presence, config, guild, member)
		if err != nil {
			log.WithError(err).Error("Error checking presence")
			errs++
			continue
		}
	}
}

func HandleGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate, client *redis.Client) {
	config, err := GetConfig(client, m.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	presence, err := s.State.Presence(m.GuildID, m.User.ID)
	if err != nil {
		log.WithError(err).Warn("Presence not, found. Most likely offline?")
		return
	}

	guild, err := s.State.Guild(m.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving guild from state")
		return
	}

	err = CheckPresence(client, presence, config, guild, m.Member)
	if err != nil {
		log.WithError(err).Error("Failed checking presence")
	}
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	config, err := GetConfig(client, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

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

		err = CheckPresence(client, p, config, g.Guild, member)

		if err != nil {
			log.WithError(err).Error("Failed checking presence")
		}
	}
}

func HandlePresenceUpdate(s *discordgo.Session, p *discordgo.PresenceUpdate, client *redis.Client) {
	config, err := GetConfig(client, p.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving streaming config")
		return
	}

	guild, err := s.State.Guild(p.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving guild from state")
		return
	}

	member, err := common.GetGuildMember(common.BotSession, p.GuildID, p.User.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving member")
		return
	}

	err = CheckPresence(client, p.Presence, config, guild, member)
	if err != nil {
		log.WithError(err).Error("Failed checking presence")
	}
}

func CheckPresence(client *redis.Client, p *discordgo.Presence, config *Config, guild *discordgo.Guild, member *discordgo.Member) error {

	// Now the real fun starts
	// Either add or remove the stream
	if p.Status != "offline" && p.Game != nil && p.Game.URL != "" {
		// Streaming

		// Only do these checks here to ensure we cleanup the user from the streaming set
		// even if the plugin was disabled or the user ended up on the ignored roles
		if !config.Enabled {
			RemoveStreaming(client, config, guild, member)
			return nil
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
				RemoveStreaming(client, config, guild, member)
				return nil
			}
		}

		if config.IgnoreRole != "" {
			for _, role := range member.Roles {
				// We ignore people with this role.. :')
				if role == config.IgnoreRole {
					RemoveStreaming(client, config, guild, member)
					return nil
				}
			}
		}

		// Was already marked as streaming before if we added 0 elements
		if num, _ := client.Cmd("SADD", "currenly_streaming:"+guild.ID, member.User.ID).Int(); num == 0 {
			return nil
		}

		// Send the streaming announcement if enabled
		if config.AnnounceChannel != "" && config.AnnounceMessage != "" {
			SendStreamingAnnouncement(config, guild, member, p)
		}

		if config.GiveRole != "" {
			GiveStreamingRole(member, config.GiveRole, guild)
		}

	} else {
		// Not streaming
		RemoveStreaming(client, config, guild, member)
	}

	return nil
}

func RemoveStreaming(client *redis.Client, config *Config, guild *discordgo.Guild, member *discordgo.Member) {
	// Was not streaming before if we removed 0 elements
	if num, _ := client.Cmd("SREM", "currenly_streaming:"+guild.ID, member.User.ID).Int(); num == 0 {
		return
	}

	RemoveStreamingRole(member, config.GiveRole, guild)
}

func SendStreamingAnnouncement(config *Config, guild *discordgo.Guild, member *discordgo.Member, p *discordgo.Presence) {
	foundChannel := false
	for _, v := range guild.Channels {
		if v.ID == config.AnnounceChannel {
			foundChannel = true
		}
	}

	if !foundChannel {
		return
	}

	templateData := map[string]interface{}{
		"user":   member.User,
		"User":   member.User,
		"Server": guild,
		"server": guild,
		"URL":    p.Game.URL,
		"url":    p.Game.URL,
	}

	out, err := common.ParseExecuteTemplate(config.AnnounceMessage, templateData)
	if err != nil {
		log.WithError(err).Error("Failed executing template")
		return
	}

	common.BotSession.ChannelMessageSend(config.AnnounceChannel, out)
}

func GiveStreamingRole(member *discordgo.Member, role string, guild *discordgo.Guild) {
	// Ensure the role exists
	found := false
	for _, v := range guild.Roles {
		if v.ID == role {
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Check if this member already has the role
	for _, r := range member.Roles {
		if r == role {
			return
		}
	}

	newRoles := make([]string, len(member.Roles)+1)
	copy(newRoles, member.Roles)
	newRoles[len(newRoles)-1] = role

	err := common.BotSession.GuildMemberEdit(guild.ID, member.User.ID, newRoles)
	if err != nil {
		log.WithError(err).Error("Error adding streaming role")
	}
}

func RemoveStreamingRole(member *discordgo.Member, role string, guild *discordgo.Guild) {
	found := false
	newRoles := make([]string, 0)
	for _, r := range member.Roles {
		if r == role {
			found = true
		} else {
			newRoles = append(newRoles, r)
		}
	}

	// Does not have role
	if !found {
		return
	}

	err := common.BotSession.GuildMemberEdit(guild.ID, member.User.ID, newRoles)
	if err != nil {
		log.WithError(err).Error("Error removing streaming role")
	}
}
