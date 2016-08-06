package serverstats

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

func (p *Plugin) InitBot() {
	bot.Session.AddHandler(bot.CustomGuildMemberAdd(HandleMemberAdd))
	bot.Session.AddHandler(bot.CustomGuildMemberRemove(HandleMemberRemove))
	bot.Session.AddHandler(bot.CustomMessageCreate(HandleMessageCreate))

	bot.Session.AddHandler(bot.CustomPresenceUpdate(HandlePresenceUpdate))
	bot.Session.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
	bot.Session.AddHandler(bot.CustomReady(HandleReady))

	bot.CommandSystem.RegisterCommands(&bot.CustomCommand{
		Key: "stats_settings_public:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Stats",
			Description: "Shows server stats",
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				channel, err := bot.Session.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				guild, err := bot.Session.State.Guild(channel.GuildID)
				if err != nil {
					return err
				}

				client, err := common.RedisPool.Get()
				if err != nil {
					return err
				}
				defer common.RedisPool.Put(client)

				stats, err := RetrieveFullStats(client, guild.ID)
				if err != nil {
					return err
				}

				total := 0
				for _, c := range stats.ChannelsHour {
					total += c.Count
				}

				header := fmt.Sprintf("Server stats for **%s** *(Also viewable at %s/public/%s/stats)* ", guild.Name, common.Conf.Host, guild.ID)
				body := fmt.Sprintf("Members joined last 24h: **%d**\n", stats.JoinedDay)
				body += fmt.Sprintf("Members Left last 24h: **%d**\n", stats.LeftDay)
				body += fmt.Sprintf("Members online: **%d**\n", stats.Online)
				body += fmt.Sprintf("Total Members: **%d**\n", stats.TotalMembers)
				body += fmt.Sprintf("Messages last 24h: **%d**\n", total)

				bot.Session.ChannelMessageSend(m.ChannelID, header+"\n\n"+body)
				return nil
			},
		},
	})

}

func HandleReady(s *discordgo.Session, r *discordgo.Ready, client *redis.Client) {
	for _, guild := range r.Guilds {
		if guild.Unavailable != nil && *guild.Unavailable {
			continue
		}
		err := ApplyPresences(client, guild.ID, guild.Presences)
		if err != nil {
			log.Println("Failed applying presences:", err)
		}
		err = LoadGuildMembers(client, guild.ID)
		if err != nil {
			log.Println("Failed loading guild members:", err)
		}
	}
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	err := ApplyPresences(client, g.ID, g.Presences)
	if err != nil {
		log.Println("Failed applying presences:", err)
	}
	err = LoadGuildMembers(client, g.ID)
	if err != nil {
		log.Println("Failed loading guild members:", err)
	}
}

func HandleMemberAdd(s *discordgo.Session, g *discordgo.GuildMemberAdd, client *redis.Client) {
	err := client.Cmd("ZADD", "guild_stats_members_joined_day:"+g.GuildID, time.Now().Unix(), g.User.ID).Err
	if err != nil {
		log.Println("Failed adding member to stats", err)
	}

	err = client.Cmd("INCR", "guild_stats_num_members:"+g.GuildID).Err
	if err != nil {
		log.Println("Failed Increasing members", err)
	}
}

func HandlePresenceUpdate(s *discordgo.Session, p *discordgo.PresenceUpdate, client *redis.Client) {
	if p.Status == "" { // Not a status update
		return
	}

	var err error
	if p.Status == "offline" {
		err = client.Cmd("SREM", "guild_stats_online:"+p.GuildID, p.User.ID).Err
	} else {
		err = client.Cmd("SADD", "guild_stats_online:"+p.GuildID, p.User.ID).Err
	}

	if err != nil {
		log.Println("Failed updating a presence", err)
	}
}

func HandleMemberRemove(s *discordgo.Session, g *discordgo.GuildMemberRemove, client *redis.Client) {
	err := client.Cmd("ZADD", "guild_stats_members_left_day:"+g.GuildID, time.Now().Unix(), g.User.ID).Err
	if err != nil {
		log.Println("Failed adding member to stats", err)
	}

	err = client.Cmd("DECR", "guild_stats_num_members:"+g.GuildID).Err
	if err != nil {
		log.Println("Failed decreasing members", err)
	}
}

func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate, client *redis.Client) {
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.Println("Error retrieving channel from state", err)
		return
	}
	err = client.Cmd("ZADD", "guild_stats_msg_channel_day:"+channel.GuildID, time.Now().Unix(), channel.ID+":"+m.ID+":"+m.Author.ID).Err
	if err != nil {
		log.Println("Failed adding member to stats", err)
	}
}

func ApplyPresences(client *redis.Client, guildID string, presences []*discordgo.Presence) error {
	client.Append("DEL", "guild_stats_online:"+guildID)
	count := 1
	for _, p := range presences {
		if p.Status == "offline" {
			continue
		}
		count++
		client.Append("SADD", "guild_stats_online:"+guildID, p.User.ID)
	}

	replies := common.GetRedisReplies(client, count)

	for _, r := range replies {
		if r.Err != nil {
			return r.Err
		}
	}

	return nil
}

func LoadGuildMembers(client *redis.Client, guildID string) error {
	started := time.Now()
	err := client.Cmd("SET", "guild_stats_num_members:"+guildID, 0).Err

	if err != nil {
		return err
	}

	members, err := dutil.GetAllGuildMembers(common.BotSession, guildID)
	if err != nil {
		return err
	}

	err = client.Cmd("INCRBY", "guild_stats_num_members:"+guildID, len(members)).Err
	log.Println("Finished loading", len(members), "guild members in", time.Since(started))
	return err
}
