package serverstats

import (
	"context"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

func MarkGuildAsToBeChecked(guildID int64) {
	common.RedisPool.Do(radix.FlatCmd(nil, "SADD", "serverstats_active_guilds", guildID))
}

var (
	_ bot.BotInitHandler       = (*Plugin)(nil)
	_ commands.CommandProvider = (*Plugin)(nil)
)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(HandleMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandler(HandleMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandler(HandleMessageCreate, eventsystem.EventMessageCreate)
	eventsystem.AddHandler(HandleGuildCreate, eventsystem.EventGuildCreate)

	pubsub.AddHandler("server_stats_invalidate_cache", func(evt *pubsub.Event) {
		gs := bot.State.Guild(true, evt.TargetGuildInt)
		if gs != nil {
			gs.UserCacheDel(true, CacheKeyConfig)
		}
	}, nil)
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryTool,
		Cooldown:      5,
		Name:          "Stats",
		Description:   "Shows server stats (if public stats are enabled)",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			config, err := GetConfig(data.Context(), data.GS.ID)
			if err != nil {
				return nil, errors.WithMessage(err, "getconfig")
			}

			if !config.Public {
				return fmt.Sprintf("Stats are set to private on this server, this can be changed in the control panel on <https://%s>", common.Conf.Host), nil
			}

			stats, err := RetrieveFullStats(data.GS.ID)
			if err != nil {
				return nil, errors.WithMessage(err, "retrievefullstats")
			}

			total := int64(0)
			for _, c := range stats.ChannelsHour {
				total += c.Count
			}

			embed := &discordgo.MessageEmbed{
				Title:       "Server stats",
				Description: fmt.Sprintf("[Click here to open in browser](https://%s/public/%d/stats)", common.Conf.Host, data.GS.ID),
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
	})

}

func HandleGuildCreate(evt *eventsystem.EventData) {
	g := evt.GuildCreate()

	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", "guild_stats_num_members:"+discordgo.StrID(g.ID), g.MemberCount))
	if err != nil {
		log.WithError(err).Error("Failed Settings member count")
	}
}

func HandleMemberAdd(evt *eventsystem.EventData) {
	g := evt.GuildMemberAdd()

	err := common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", "guild_stats_members_joined_day:"+discordgo.StrID(g.GuildID), time.Now().Unix(), g.User.ID))
	if err != nil {
		log.WithError(err).Error("Failed adding member to stats")
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "INCR", "guild_stats_num_members:"+discordgo.StrID(g.GuildID)))
	if err != nil {
		log.WithError(err).Error("Failed Increasing members")
	}
}

func HandleMemberRemove(evt *eventsystem.EventData) {
	g := evt.GuildMemberRemove()

	err := common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", "guild_stats_members_left_day:"+discordgo.StrID(g.GuildID), time.Now().Unix(), g.User.ID))
	if err != nil {
		log.WithError(err).Error("Failed adding member to stats")
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "DECR", "guild_stats_num_members:"+discordgo.StrID(g.GuildID)))
	if err != nil {
		log.WithError(err).Error("Failed decreasing members")
	}
}

func HandleMessageCreate(evt *eventsystem.EventData) {

	m := evt.MessageCreate()
	if m.GuildID == 0 {
		return // private channel
	}

	channel := bot.State.Channel(true, m.ChannelID)
	if channel == nil {
		log.WithField("channel", m.ChannelID).Warn("Channel not in state")
		return
	}

	config, err := BotCachedFetchGuildConfig(evt.Context(), channel.Guild)
	if err != nil {
		log.WithError(err).WithField("guild", channel.Guild.ID).Error("Failed retrieving config")
		return
	}

	if common.ContainsInt64Slice(config.ParsedChannels, channel.ID) {
		return
	}

	val := channel.StrID() + ":" + discordgo.StrID(m.ID) + ":" + discordgo.StrID(m.Author.ID)
	err = common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", "guild_stats_msg_channel_day:"+channel.Guild.StrID(), time.Now().Unix(), val))
	if err != nil {
		log.WithError(err).Error("Failed adding member to stats")
	}

	MarkGuildAsToBeChecked(channel.Guild.ID)
}

type CacheKey int

const (
	CacheKeyConfig CacheKey = iota
)

func BotCachedFetchGuildConfig(ctx context.Context, gs *dstate.GuildState) (*ServerStatsConfig, error) {
	v, err := gs.UserCacheFetch(true, CacheKeyConfig, func() (interface{}, error) {
		return GetConfig(ctx, gs.ID)
	})

	if err != nil {
		return nil, err
	}

	return v.(*ServerStatsConfig), nil
}
