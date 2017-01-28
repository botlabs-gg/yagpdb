package serverstats

import (
	"context"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"time"
)

func (p *Plugin) InitBot() {
	bot.AddHandler(bot.RedisWrapper(HandleMemberAdd), bot.EventGuildMemberAdd)
	bot.AddHandler(bot.RedisWrapper(HandleMemberRemove), bot.EventGuildMemberRemove)
	bot.AddHandler(bot.RedisWrapper(HandleMessageCreate), bot.EventMessageCreate)

	bot.AddHandler(bot.RedisWrapper(HandlePresenceUpdate), bot.EventPresenceUpdate)
	bot.AddHandler(bot.RedisWrapper(HandleGuildCreate), bot.EventGuildCreate)
	bot.AddHandler(bot.RedisWrapper(HandleReady), bot.EventReady)

	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryTool,
		Cooldown:      10,
		Command: &commandsystem.Command{
			Name:        "Stats",
			Description: "Shows server stats (if public stats are enabled)",
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				config, err := GetConfig(data.Context(), data.Guild.ID())
				if err != nil {
					return "Failed retreiving guild config", err
				}

				if !config.Public {
					return "Stats are set to private on this server, this can be changed in the control panel on <http://yagpdb.xyz>", nil
				}

				stats, err := RetrieveFullStats(data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), data.Guild.ID())
				if err != nil {
					return "Error retrieving stats", err
				}

				total := 0
				for _, c := range stats.ChannelsHour {
					total += c.Count
				}

				embed := &discordgo.MessageEmbed{
					Title:       "Server stats",
					Description: fmt.Sprintf("[Click here to open in browser](https://%s/public/%s/stats)", common.Conf.Host, data.Guild.ID()),
					Fields: []*discordgo.MessageEmbedField{
						&discordgo.MessageEmbedField{Name: "Members joined 24h", Value: fmt.Sprint(stats.JoinedDay), Inline: true},
						&discordgo.MessageEmbedField{Name: "Members Left 24h", Value: fmt.Sprint(stats.LeftDay), Inline: true},
						&discordgo.MessageEmbedField{Name: "Total Messages 24h", Value: fmt.Sprint(total), Inline: true},
						&discordgo.MessageEmbedField{Name: "Members Online", Value: fmt.Sprint(stats.Online), Inline: true},
						&discordgo.MessageEmbedField{Name: "Total Members", Value: fmt.Sprint(stats.TotalMembers), Inline: true},
					},
				}

				return embed, nil
			},
		},
	})

}

func HandleReady(ctx context.Context, evt interface{}) {
	r := evt.(*discordgo.Ready)
	client := bot.ContextRedis(ctx)

	for _, guild := range r.Guilds {
		if guild.Unavailable {
			continue
		}

		err := ApplyPresences(client, guild.ID, guild.Presences)
		if err != nil {
			log.WithError(err).Error("Failed applying presences")
		}
	}
}

func HandleGuildCreate(ctx context.Context, evt interface{}) {
	g := evt.(*discordgo.GuildCreate)
	client := bot.ContextRedis(ctx)

	err := client.Cmd("SET", "guild_stats_num_members:"+g.ID, g.MemberCount).Err
	if err != nil {
		log.WithError(err).Error("Failed Settings member count")
	}
	log.WithField("guild", g.ID).WithField("g_name", g.Name).WithField("member_count", g.MemberCount).Info("Set member count")

	err = ApplyPresences(client, g.ID, g.Presences)
	if err != nil {
		log.WithError(err).Error("Failed applying presences")
	}
}

func HandleMemberAdd(ctx context.Context, evt interface{}) {
	g := evt.(*discordgo.GuildMemberAdd)
	client := bot.ContextRedis(ctx)

	err := client.Cmd("ZADD", "guild_stats_members_joined_day:"+g.GuildID, time.Now().Unix(), g.User.ID).Err
	if err != nil {
		log.WithError(err).Error("Failed adding member to stats")
	}

	err = client.Cmd("INCR", "guild_stats_num_members:"+g.GuildID).Err
	if err != nil {
		log.WithError(err).Error("Failed Increasing members")
	}
}

func HandlePresenceUpdate(ctx context.Context, evt interface{}) {
	p := evt.(*discordgo.PresenceUpdate)
	client := bot.ContextRedis(ctx)

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
		log.WithError(err).Error("Failed updating a presence")
	}
}

func HandleMemberRemove(ctx context.Context, evt interface{}) {
	g := evt.(*discordgo.GuildMemberRemove)
	client := bot.ContextRedis(ctx)

	err := client.Cmd("ZADD", "guild_stats_members_left_day:"+g.GuildID, time.Now().Unix(), g.User.ID).Err
	if err != nil {
		log.WithError(err).Error("Failed adding member to stats")
	}

	err = client.Cmd("DECR", "guild_stats_num_members:"+g.GuildID).Err
	if err != nil {
		log.WithError(err).Error("Failed decreasing members")
	}
}

func HandleMessageCreate(ctx context.Context, evt interface{}) {

	m := evt.(*discordgo.MessageCreate)
	client := bot.ContextRedis(ctx)
	channel := bot.State.Channel(true, m.ChannelID)

	if channel.IsPrivate() {
		return
	}

	config, err := GetConfig(ctx, channel.Guild.ID())
	if err != nil {
		log.WithError(err).WithField("guild", channel.Guild.ID()).Error("Failed retrieving config")
		return
	}

	for _, v := range config.ParsedChannels {
		if channel.ID() == v {
			return
		}
	}

	if channel == nil {
		ch, err := common.BotSession.Channel(m.ChannelID)
		if err != nil {
			log.WithField("channel", m.ChannelID).WithField("msg", m.Content).Error("nil channel")
		} else {
			log.Println("Found channel but nil in state? ", m.ChannelID, ch.Name, ch.GuildID)
		}
		return
	}

	if channel.IsPrivate() {
		return
	}

	err = client.Cmd("ZADD", "guild_stats_msg_channel_day:"+channel.Guild.ID(), time.Now().Unix(), channel.ID()+":"+m.ID+":"+m.Author.ID).Err
	if err != nil {
		log.WithError(err).Error("Failed adding member to stats")
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
