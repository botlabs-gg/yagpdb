package serverstats

import (
	"fmt"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

func (p *Plugin) InitBot() {
	common.BotSession.AddHandler(bot.CustomGuildMemberAdd(HandleMemberAdd))
	common.BotSession.AddHandler(bot.CustomGuildMemberRemove(HandleMemberRemove))
	common.BotSession.AddHandler(bot.CustomMessageCreate(HandleMessageCreate))

	common.BotSession.AddHandler(bot.CustomPresenceUpdate(HandlePresenceUpdate))
	common.BotSession.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(bot.CustomReady(HandleReady))

	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		Key:      "stats_settings_public:",
		Cooldown: 10,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Stats",
			Description: "Shows server stats",
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			stats, err := RetrieveFullStats(client, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving stats", err
			}

			total := 0
			for _, c := range stats.ChannelsHour {
				total += c.Count
			}

			header := fmt.Sprintf("Server stats for **%s** *(Also viewable at %s/public/%s/stats)* ", parsed.Guild.Name, common.Conf.Host, parsed.Guild.ID)
			body := fmt.Sprintf("Members joined last 24h: **%d**\n", stats.JoinedDay)
			body += fmt.Sprintf("Members Left last 24h: **%d**\n", stats.LeftDay)
			body += fmt.Sprintf("Members online: **%d**\n", stats.Online)
			body += fmt.Sprintf("Total Members: **%d**\n", stats.TotalMembers)
			body += fmt.Sprintf("Messages last 24h: **%d**\n", total)

			return header + "\n\n" + body, nil
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

	_, err := common.GetRedisReplies(client, count)
	return err
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

	// Load all members in memory, this might cause issues in the future, who knows /shrug
	for _, v := range members {
		v.GuildID = guildID
		err = common.BotSession.State.MemberAdd(v)
		if err != nil {
			log.Println("Error", err)
		}
	}

	err = client.Cmd("INCRBY", "guild_stats_num_members:"+guildID, len(members)).Err
	log.Println("Finished loading", len(members), "guild members in", time.Since(started))
	return err
}
