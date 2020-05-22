package serverstats

import (
	"context"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/serverstats/messagestatscollector"
	"github.com/jonas747/yagpdb/web"
	"github.com/mediocregopher/radix/v3"
)

func MarkGuildAsToBeChecked(guildID int64) {
	common.RedisPool.Do(radix.FlatCmd(nil, "SADD", "serverstats_active_guilds", guildID))
}

var (
	_                   bot.BotInitHandler       = (*Plugin)(nil)
	_                   commands.CommandProvider = (*Plugin)(nil)
	msgStatsCollector   *messagestatscollector.Collector
	memberSatatsUpdater *serverMemberStatsUpdater
)

func (p *Plugin) BotInit() {
	msgStatsCollector = messagestatscollector.NewCollector(logger, time.Minute)
	memberSatatsUpdater = newServerMemberStatsUpdater()
	go memberSatatsUpdater.run()

	eventsystem.AddHandlerAsyncLastLegacy(p, handleUpdateMemberStats, eventsystem.EventGuildMemberAdd, eventsystem.EventGuildMemberRemove, eventsystem.EventGuildCreate)

	eventsystem.AddHandlerAsyncLast(p, eventsystem.RequireCSMW(HandleMessageCreate), eventsystem.EventMessageCreate)

	pubsub.AddHandler("server_stats_invalidate_cache", func(evt *pubsub.Event) {
		gs := bot.State.Guild(true, evt.TargetGuildInt)
		if gs != nil {
			gs.UserCacheDel(CacheKeyConfig)
		}
	}, nil)

	go p.runOnlineUpdater()
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, &commands.YAGCommand{
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
				return fmt.Sprintf("Stats are set to private on this server, this can be changed in the control panel on <https://%s>", common.ConfHost.GetString()), nil
			}

			stats, err := RetrieveDailyStats(time.Now(), data.GS.ID)
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

func handleUpdateMemberStats(evt *eventsystem.EventData) {
	select {
	case memberSatatsUpdater.incoming <- evt:
	default:
		go func() {
			memberSatatsUpdater.incoming <- evt
		}()
	}
}

func HandleMessageCreate(evt *eventsystem.EventData) (retry bool, err error) {

	m := evt.MessageCreate()
	if m.GuildID == 0 || m.Author == nil || m.Author.Bot {
		return // private channel
	}

	channel := evt.CS()

	config, err := BotCachedFetchGuildConfig(evt.Context(), channel.Guild)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if common.ContainsInt64Slice(config.ParsedChannels, channel.ID) {
		return false, nil
	}

	msgStatsCollector.MsgEvtChan <- m.Message
	return false, nil
}

type CacheKey int

const (
	CacheKeyConfig CacheKey = iota
)

func BotCachedFetchGuildConfig(ctx context.Context, gs *dstate.GuildState) (*ServerStatsConfig, error) {
	v, err := gs.UserCacheFetch(CacheKeyConfig, func() (interface{}, error) {
		return GetConfig(ctx, gs.ID)
	})

	if err != nil {
		return nil, err
	}

	return v.(*ServerStatsConfig), nil
}

func (p *Plugin) runOnlineUpdater() {
	time.Sleep(time.Minute * 10) // relieve startup preasure

	ticker := time.NewTicker(time.Second * 10)
	state := bot.State

	var guildsToCheck []*dstate.GuildState
	var i int
	var numToCheckPerRun int

	for {
		select {
		case <-ticker.C:
		}

		if len(guildsToCheck) < 0 || i >= len(guildsToCheck) {
			// Copy the list of guilds so that we dont need to keep the entire state locked

			i = 0
			guildsToCheck = state.GuildsSlice(true)

			// Hit each guild once per hour more or less
			numToCheckPerRun = len(guildsToCheck) / 250
			if numToCheckPerRun < 1 {
				numToCheckPerRun = 1
			}
		}

		started := time.Now()

		totalCounts := make(map[int64][2]int)

		checkedThisRound := 0
		for ; i < len(guildsToCheck) && checkedThisRound < numToCheckPerRun; i++ {
			g := guildsToCheck[i]
			online, total := p.checkGuildOnlineCount(g)

			totalCounts[g.ID] = [2]int{online, total}
			checkedThisRound++
		}

		t := RoundHour(time.Now())

		tx, err := common.PQ.Begin()
		if err != nil {
			logger.WithError(err).Error("[serverstats] failed starting online count transaction")
			continue
		}

		for g, counts := range totalCounts {
			_, err := tx.Exec(`INSERT INTO server_stats_hourly_periods_misc  (guild_id, t, num_members, joins, leaves, max_online, max_voice)
VALUES ($1, $2, $3, 0, 0, $4, 0)
ON CONFLICT (guild_id, t)
DO UPDATE SET 
max_online = GREATEST (server_stats_hourly_periods_misc.max_online, $4)
`, g, t, counts[1], counts[0]) // update clause vars

			if err != nil {
				logger.WithError(err).WithField("guild", g).Error("failed checking guild online count")
				tx.Rollback()
				break
			}
		}

		err = tx.Commit()
		if err != nil {
			logger.WithError(err).Error("failed comitting online counts")
		}

		if time.Since(started) > time.Second {
			logger.Warnf("Tok %s to update online counts of %d guilds", time.Since(started).String(), checkedThisRound)
		}
	}
}

func (p *Plugin) checkGuildOnlineCount(guild *dstate.GuildState) (online int, total int) {

	guild.RLock()
	total = guild.Guild.MemberCount
	for _, v := range guild.Members {
		if v.PresenceSet && v.PresenceStatus != dstate.StatusOffline {
			online++
		}
	}
	guild.RUnlock()

	return online, total
}
