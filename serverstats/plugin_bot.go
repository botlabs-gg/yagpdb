package serverstats

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
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

	go p.runOnlineUpdater()
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

			stats, err := RetrieveDailyStats(data.GS.ID)
			if err != nil {
				return nil, errors.WithMessage(err, "retrievefullstats")
			}

			total := int64(0)
			for _, c := range stats.ChannelMessages {
				total += c.Count
			}

			embed := &discordgo.MessageEmbed{
				Title:       "Server stats",
				Description: fmt.Sprintf("[Click here to open in browser](%s/public/%d/stats)", web.BaseURL(), data.GS.ID),
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

	SetUpdateMemberStatsPeriod(g.ID, 0, g.MemberCount)
}

func HandleMemberAdd(evt *eventsystem.EventData) {
	g := evt.GuildMemberAdd()

	gs := bot.State.Guild(true, g.GuildID)
	if gs == nil {
		return
	}

	gs.RLock()
	mc := gs.Guild.MemberCount
	gs.RUnlock()

	SetUpdateMemberStatsPeriod(g.GuildID, 1, mc)

	common.LogIgnoreError(common.RedisPool.Do(radix.FlatCmd(nil, "SADD", RedisKeyGuildMembersChanged, g.GuildID)),
		"[serverstats] failed marking guildmembers changed", log.Fields{"guild": g.GuildID})
}

func HandleMemberRemove(evt *eventsystem.EventData) {
	g := evt.GuildMemberRemove()

	gs := bot.State.Guild(true, g.GuildID)
	if gs == nil {
		return
	}

	gs.RLock()
	mc := gs.Guild.MemberCount
	gs.RUnlock()

	SetUpdateMemberStatsPeriod(g.GuildID, -1, mc)

	common.LogIgnoreError(common.RedisPool.Do(radix.FlatCmd(nil, "SADD", RedisKeyGuildMembersChanged, g.GuildID)),
		"[serverstats] failed marking guildmembers changed", log.Fields{"guild": g.GuildID})
}

func SetUpdateMemberStatsPeriod(guildID int64, memberIncr int, numMembers int) {
	joins := 0
	leaves := 0
	if memberIncr > 0 {
		joins = memberIncr
	} else if memberIncr < 0 {
		leaves = -memberIncr
	}

	// round to current hour
	t := RoundHour(time.Now())

	_, err := common.PQ.Exec(`INSERT INTO server_stats_member_periods  (guild_id, created_at, num_members, joins, leaves, max_online)
VALUES ($1, $2, $3, $4, $5, 0)
ON CONFLICT (guild_id, created_at)
DO UPDATE SET 
joins = server_stats_member_periods.joins + $4, 
leaves = server_stats_member_periods.leaves + $5, 
num_members = server_stats_member_periods.num_members + $6;`, guildID, t, numMembers, joins, leaves, memberIncr) // update clause vars

	if err != nil {
		log.WithError(err).Error("[serverstats] failed setting member stats period")
	}

}

func HandleMessageCreate(evt *eventsystem.EventData) {

	m := evt.MessageCreate()
	if m.GuildID == 0 || m.Author == nil || m.Author.Bot {
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

func (p *Plugin) runOnlineUpdater() {

	ticker := time.NewTicker(time.Second)
	state := bot.State

	var guildsToCheck []*dstate.GuildState
	var i int
	var numToCheckPerRun int

OUTER:
	for {
		select {
		case <-ticker.C:
		}

		if len(guildsToCheck) < 0 || i >= len(guildsToCheck) {
			// Copy the list of guilds so that we dont need to keep the entire state locked

			i = 0
			guildsToCheck = state.GuildsSlice(true)

			// Hit each guild once per hour more or less
			numToCheckPerRun = len(guildsToCheck) / 2500
			if numToCheckPerRun < 1 {
				numToCheckPerRun = 1
			}
		}

		started := time.Now()

		tx, err := common.PQ.Begin()
		if err != nil {
			log.WithError(err).Error("[serverstats] failed starting online count transaction")
			continue
		}

		checkedThisRound := 0
		for ; i < len(guildsToCheck) && checkedThisRound < numToCheckPerRun; i++ {
			g := guildsToCheck[i]
			err := p.checkGuildOnlineCount(g, tx)
			if err != nil {
				log.WithError(err).WithField("guild", g.ID).Error("[serverstats] failed checking guild online count")
				tx.Rollback()
				continue OUTER
			}

			checkedThisRound++
		}

		err = tx.Commit()
		if err != nil {
			log.WithError(err).Error("[serverstats] failed comitting online counts")
		}

		if time.Since(started) > time.Millisecond*250 {
			log.Warnf("Tok %s to update online counts of %d guilds", time.Since(started).String(), checkedThisRound)
		}
	}
}

func (p *Plugin) checkGuildOnlineCount(guild *dstate.GuildState, tx *sql.Tx) error {
	online := 0
	totalMembers := 0

	guild.RLock()
	totalMembers = guild.Guild.MemberCount
	for _, v := range guild.Members {
		if v.PresenceSet && v.PresenceStatus != dstate.StatusOffline {
			online++
		}
	}
	guild.RUnlock()

	t := RoundHour(time.Now())

	_, err := tx.Exec(`INSERT INTO server_stats_member_periods  (guild_id, created_at, num_members, joins, leaves, max_online)
VALUES ($1, $2, $3, 0, 0, $4)
ON CONFLICT (guild_id, created_at)
DO UPDATE SET 
max_online = GREATEST (server_stats_member_periods.max_online, $4)
`, guild.ID, t, totalMembers, online) // update clause vars

	return err
}
