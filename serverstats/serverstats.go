package serverstats

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

type Plugin struct {
	stopStatsLoop chan *sync.WaitGroup
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Server Stats",
		SysName:  "server_stats",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.InitSchema(DBSchema, "serverstats")

	plugin := &Plugin{
		stopStatsLoop: make(chan *sync.WaitGroup),
	}
	common.RegisterPlugin(plugin)
}

type ChannelStats struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type DailyStats struct {
	ChannelMessages map[string]*ChannelStats `json:"channels_messages"`
	JoinedDay       int                      `json:"joined_day"`
	LeftDay         int                      `json:"left_day"`
	Online          int                      `json:"online_now"`
	TotalMembers    int                      `json:"total_members_now"`
}

func RetrieveDailyStats(guildID int64) (*DailyStats, error) {
	// Query the short term stats and the long term stats
	// TODO: If we start moving them over in between we will get somehwat incorrect stats
	// not sure how to fix other than locking

	stats, err := RetrieveRedisStats(guildID)
	if err != nil {
		return nil, err
	}

	// rows, err := ServerStatsPeriodStore.FindAll(models.NewStatsPeriodQuery().FindByGuildID(
	// 	kallax.Eq, guildID).FindByStarted(kallax.Gt))

	messageStatsRows, err := models.ServerStatsPeriods(qm.Where("guild_id = ?", guildID), qm.Where("started > ?", time.Now().Add(time.Hour*-24))).AllG(context.Background())
	if err != nil {
		return nil, err
	}

	// Merge the stats togheter
	for _, period := range messageStatsRows {
		stringedChannel := strconv.FormatInt(period.ChannelID.Int64, 10)
		if st, ok := stats.ChannelMessages[stringedChannel]; ok {
			st.Count += period.Count.Int64
		} else {
			stats.ChannelMessages[stringedChannel] = &ChannelStats{
				Name:  stringedChannel,
				Count: period.Count.Int64,
			}
		}
	}

	t := RoundHour(time.Now())
	memberStatsRows, err := models.ServerStatsMemberPeriods(
		models.ServerStatsMemberPeriodWhere.GuildID.EQ(guildID),
		qm.OrderBy("id desc"), qm.Limit(25)).AllG(context.Background())

	// Sum the stats
	for i, v := range memberStatsRows {
		if i == 0 {
			stats.TotalMembers = int(v.NumMembers)
			if v.CreatedAt.UTC() == t.UTC() {
				continue
			}
		}

		if t.Sub(v.CreatedAt) > time.Hour*25 {
			break
		}

		stats.JoinedDay += int(v.Joins)
		stats.LeftDay += int(v.Leaves)
	}

	return stats, nil
}

func RetrieveRedisStats(guildID int64) (*DailyStats, error) {
	now := time.Now()
	yesterday := now.Add(time.Hour * -24)
	unixYesterday := discordgo.StrID(yesterday.Unix())

	var messageStatsRaw []string

	err := common.RedisPool.Do(retryableredis.Cmd(&messageStatsRaw, "ZRANGEBYSCORE", RedisKeyChannelMessages(guildID), unixYesterday, "+inf"))
	if err != nil {
		return nil, err
	}

	online, err := botrest.GetOnlineCount(guildID)
	if err != nil {
		if botrest.BotIsRunning() {
			logger.WithError(err).Error("Failed fetching online count")
		}
	}

	channelResult, err := parseMessageStats(messageStatsRaw, guildID)
	if err != nil {
		return nil, err
	}

	stats := &DailyStats{
		ChannelMessages: channelResult,
		Online:          int(online),
	}

	return stats, nil
}

func parseMessageStats(raw []string, guildID int64) (map[string]*ChannelStats, error) {

	channelResult := make(map[string]*ChannelStats)
	for _, result := range raw {
		split := strings.Split(result, ":")
		if len(split) < 2 {

			logger.WithFields(logrus.Fields{
				"guild":  guildID,
				"result": result,
			}).Error("Invalid message stats")

			continue
		}
		channelID := split[0]

		stats, ok := channelResult[channelID]
		if ok {
			stats.Count++
		} else {
			name := channelID
			channelResult[channelID] = &ChannelStats{
				Name:  name,
				Count: 1,
			}
		}
	}
	return channelResult, nil
}

// ServerStatsConfig represents a configuration for a server
// reason we dont reference the model directly is because i need to figure out a way to
// migrate them over to the new schema, painlessly.
type ServerStatsConfig struct {
	Public         bool
	IgnoreChannels string

	ParsedChannels []int64
}

func (s *ServerStatsConfig) ParseChannels() {
	split := strings.Split(s.IgnoreChannels, ",")
	for _, v := range split {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			s.ParsedChannels = append(s.ParsedChannels, parsed)
		}
	}
}

func configFromModel(model *models.ServerStatsConfig) *ServerStatsConfig {
	conf := &ServerStatsConfig{
		Public:         model.Public.Bool,
		IgnoreChannels: model.IgnoreChannels.String,
	}
	conf.ParseChannels()

	return conf
}

func GetConfig(ctx context.Context, GuildID int64) (*ServerStatsConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	conf, err := models.FindServerStatsConfigG(ctx, GuildID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if conf == nil {
		return &ServerStatsConfig{}, nil
	}

	return configFromModel(conf), nil
}

const (
	RedisKeyGuildMembersChanged = "servers_stats_guild_members_changed"
)

func RedisKeyChannelMessages(guildID int64) string {
	return "guild_stats_msg_channel_day:" + strconv.FormatInt(guildID, 10)
}

// RoundHour rounds a time.Time down to the hour
func RoundHour(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

type MemberChartDataPeriod struct {
	T          time.Time `json:"t"`
	Joins      int       `json:"joins"`
	Leaves     int       `json:"leaves"`
	NumMembers int       `json:"num_members"`
	MaxOnline  int       `json:"max_online"`
}

func RetrieveMemberChartStats(guildID int64, days int) ([]*MemberChartDataPeriod, error) {
	query := `select date_trunc('day', created_at), sum(joins), sum(leaves), max(num_members), max(max_online)
FROM server_stats_member_periods
WHERE guild_id=$1 
GROUP BY 1 
ORDER BY 1 DESC`

	args := []interface{}{guildID}
	if days > 0 {
		query += " LIMIT $2;"
		args = append(args, days)
	}

	rows, err := common.PQ.Query(query, args...)

	if err != nil {
		return nil, errors.Wrap(err, "pq.query")
	}

	defer rows.Close()

	var results []*MemberChartDataPeriod
	if days > 0 {
		results = make([]*MemberChartDataPeriod, days)
	} else {
		// we don't know the size
		results = make([]*MemberChartDataPeriod, 100)
	}

	for rows.Next() {
		var t time.Time
		var joins int
		var leaves int
		var numMembers int
		var maxOnline int

		err := rows.Scan(&t, &joins, &leaves, &numMembers, &maxOnline)
		if err != nil {
			return nil, errors.Wrap(err, "rows.scan")
		}

		daysOld := int(time.Since(t).Hours() / 24)

		if daysOld > days && len(results) > 0 && days > 0 {
			// only grab results within time period specified (but always grab 1 even if outside our range)
			break
		}

		if days > 0 && daysOld >= days {
			// clamp to last if we specified a time
			daysOld = days - 1
		}

		if daysOld >= len(results) {
			if daysOld > 10000 {
				continue // ignore this then, should never happen, but lets just avoid running out of memory if it does
			}

			newResults := make([]*MemberChartDataPeriod, daysOld*2)
			copy(newResults, results)
			results = newResults
		}

		results[daysOld] = &MemberChartDataPeriod{
			T:          t,
			Joins:      joins,
			Leaves:     leaves,
			NumMembers: numMembers,
			MaxOnline:  maxOnline,
		}
	}

	firstNonNullResult := -1

	// fill in the blank days
	var lastProperResult MemberChartDataPeriod
	for i := len(results) - 1; i >= 0; i-- {
		if results[i] == nil && !lastProperResult.T.IsZero() {
			cop := lastProperResult
			t := time.Now().Add(time.Hour * 24 * -time.Duration(i))
			cop.T = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, lastProperResult.T.Location())

			results[i] = &cop
		} else if results[i] != nil {
			lastProperResult = *results[i]
			lastProperResult.Joins = 0
			lastProperResult.Leaves = 0
			if firstNonNullResult == -1 {
				firstNonNullResult = i
			}
		}
	}

	// cut out nil results
	results = results[:firstNonNullResult+1]

	return results, nil
}

type MessageChartDataPeriod struct {
	T            time.Time `json:"t"`
	MessageCount int       `json:"message_count"`
}

func RetrieveMessageChartData(guildID int64, days int) ([]*MessageChartDataPeriod, error) {
	queryPre := `select date_trunc('day', started), sum(count)
FROM server_stats_periods
WHERE guild_id=$1 `
	queryPost := `
GROUP BY 1 
ORDER BY 1 DESC`

	args := []interface{}{guildID}
	if days > 0 {
		queryPre += " AND started > $2"
		args = append(args, time.Now().Add(time.Hour*24*time.Duration(-days)))
	}

	rows, err := common.PQ.Query(queryPre+queryPost, args...)

	if err != nil {
		return nil, errors.Wrap(err, "pq.query")
	}

	defer rows.Close()

	var results []*MessageChartDataPeriod
	if days > 0 {
		results = make([]*MessageChartDataPeriod, days)
	} else {
		// we don't know the size
		results = make([]*MessageChartDataPeriod, 100)
	}

	for rows.Next() {
		var t time.Time
		var count int

		err := rows.Scan(&t, &count)
		if err != nil {
			return nil, errors.Wrap(err, "rows.scan")
		}

		daysOld := int(time.Since(t).Hours() / 24)

		if daysOld >= days && days > 0 {
			// clamp to last if we specified a time
			daysOld = days - 1
		}

		if daysOld >= len(results) {
			// we don't know the size so we have to dynamically adjust
			if daysOld > 10000 {
				continue // ignore this then, should never happen, but lets just avoid running out of memory if it does
			}

			newResults := make([]*MessageChartDataPeriod, daysOld*2)
			copy(newResults, results)
			results = newResults
		}

		results[daysOld] = &MessageChartDataPeriod{
			T:            t,
			MessageCount: count,
		}
	}

	firstNonNullResult := -1

	// fill in the blank days
	var lastProperResult MessageChartDataPeriod
	for i := len(results) - 1; i >= 0; i-- {
		if results[i] == nil && !lastProperResult.T.IsZero() {
			cop := lastProperResult
			t := time.Now().Add(time.Hour * 24 * -time.Duration(i))
			cop.T = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, lastProperResult.T.Location())

			results[i] = &cop
		} else if results[i] != nil {
			lastProperResult = *results[i]
			lastProperResult.MessageCount = 0

			if firstNonNullResult == -1 {
				firstNonNullResult = i
			}
		}
	}

	// cut out nil results
	results = results[:firstNonNullResult+1]

	return results, nil
}
