package serverstats

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"log"
	"time"
)

func (p *Plugin) InitBot() {
	bot.Session.AddHandler(bot.CustomGuildMemberAdd(HandleMemberAdd))
	bot.Session.AddHandler(bot.CustomGuildMemberRemove(HandleMemberRemove))
	bot.Session.AddHandler(bot.CustomMessageCreate(HandleMessageCreate))
}

func HandleMemberAdd(s *discordgo.Session, g *discordgo.GuildMemberAdd, client *redis.Client) {
	err := client.Cmd("ZADD", "guild_stats_members_joined_day:"+g.GuildID, time.Now().Unix(), g.User.ID).Err
	if err != nil {
		log.Println("Failed adding member to stats", err)
	}
}

func HandleMemberRemove(s *discordgo.Session, g *discordgo.GuildMemberRemove, client *redis.Client) {
	err := client.Cmd("ZADD", "guild_stats_members_left_day:"+g.GuildID, time.Now().Unix(), g.User.ID).Err
	if err != nil {
		log.Println("Failed adding member to stats", err)
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
