package streaming

import (
	"fmt"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"log"
)

func (p *Plugin) InitBot() {
	common.BotSession.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(bot.CustomPresenceUpdate(HandlePresenceUpdate))

	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		Cooldown: 10,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "refreshstreaming",
			Aliases:     []string{"rfs", "updatestreaming"},
			Description: "Rechecks the streaming status of all the online people on the server, usefull if you added somone to the role",
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			config, err := GetConfig(client, parsed.Guild.ID)
			if err != nil {
				return "Failed retrieving config", err
			}
			errs := 0
			for _, presence := range parsed.Guild.Presences {

				var member *discordgo.Member

				for _, m := range parsed.Guild.Members {
					if m.User.ID == presence.User.ID {
						member = m
						break
					}
				}

				if member == nil {
					log.Println("Member not found in guild", presence.User.ID)
					errs++
					continue
				}

				err = CheckPresence(client, presence, config, parsed.Guild, member)
				if err != nil {
					log.Println("Error checking presence", err)
					errs++
					continue
				}
			}
			out := "👌"
			if errs > 0 {
				out = fmt.Sprintf("%d errors occured, contact the jonus", errs)
			}
			return out, nil
		},
	})
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	config, err := GetConfig(client, g.ID)
	if err != nil {
		log.Println("Failed retrieving config", err)
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
			log.Println("No member found :'(")
			continue
		}

		err = CheckPresence(client, p, config, g.Guild, member)

		if err != nil {
			log.Println("Failed checking presence", err)
		}
	}
}

func HandlePresenceUpdate(s *discordgo.Session, p *discordgo.PresenceUpdate, client *redis.Client) {
	config, err := GetConfig(client, p.GuildID)
	if err != nil {
		log.Println("Failed retrieving config", err)
		return
	}

	member, err := s.State.Member(p.GuildID, p.User.ID)
	if err != nil {
		log.Println("Failed retrieving member from state", err)
		return
	}

	guild, err := s.State.Guild(p.GuildID)
	if err != nil {
		log.Println("Failed retrieving guild from state", err)
	}

	err = CheckPresence(client, &p.Presence, config, guild, member)
	if err != nil {
		log.Println("Failed checking presence", err)
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
		log.Println("Failed executing template", err)
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

	member.Roles = append(member.Roles, role)
	err := common.BotSession.GuildMemberEdit(guild.ID, member.User.ID, member.Roles)
	if err != nil {
		log.Println("Error updating sreaming role", err)
	}
}

func RemoveStreamingRole(member *discordgo.Member, role string, guild *discordgo.Guild) {
	index := -1
	for k, r := range member.Roles {
		if r == role {
			index = k
			break
		}
	}

	// Does not have role
	if index == -1 {
		return
	}

	member.Roles = append(member.Roles[:index], member.Roles[index+1:]...)
	err := common.BotSession.GuildMemberEdit(guild.ID, member.User.ID, member.Roles)
	if err != nil {
		log.Println("Error updating streaming role")
	}
}
