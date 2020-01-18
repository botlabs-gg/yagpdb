package serverstats

import (
	"context"
	"strconv"
	"time"

	"emperror.dev/errors"

	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
)

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

func RetrieveDailyStats(t time.Time, guildID int64) (*DailyStats, error) {

	msgStats, err := readHourlyMessageStats(t, guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	miscStats, err := readHourlyMiscStats(t, guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	online, err := botrest.GetOnlineCount(guildID)
	if err != nil {
		logger.WithError(err).Error("Failed fetching online count")
	}

	miscStats.Online = int(online)

	miscStats.ChannelMessages = msgStats
	return miscStats, nil
}

func readHourlyMessageStats(t time.Time, guildID int64) (map[string]*ChannelStats, error) {
	// read horly message stats
	const qMsgStatsHourly = `SELECT t, channel_id, count 
FROM server_stats_hourly_periods_messages
WHERE t > ($2::timestamptz - INTERVAL '25 hours') AND guild_id = $1 AND date_trunc('hour', t) < date_trunc('hour', $2)`

	rows, err := common.PQ.Query(qMsgStatsHourly, guildID, t)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}
	defer rows.Close()

	channelStats := make(map[string]*ChannelStats)

	for rows.Next() {
		var channelID int64
		var count int64
		var t time.Time
		err = rows.Scan(&t, &channelID, &count)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		cStr := strconv.FormatInt(channelID, 10)

		if cs, ok := channelStats[cStr]; ok {
			cs.Count += count
		} else {
			channelStats[cStr] = &ChannelStats{
				Name:  cStr,
				Count: count,
			}
		}
	}

	return channelStats, nil
}

func readHourlyMiscStats(t time.Time, guildID int64) (*DailyStats, error) {
	// read the rest
	const qMiscStatsHourly = `SELECT t, num_members, max_online, joins, leaves, max_voice 
FROM server_stats_hourly_periods_misc
WHERE t > ($2::timestamptz - INTERVAL '25 hours') AND guild_id = $1 AND date_trunc('hour', t) < date_trunc('hour', $2)`

	rows, err := common.PQ.Query(qMiscStatsHourly, guildID, t)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}
	defer rows.Close()

	sum := &DailyStats{}

	for rows.Next() {
		var t time.Time
		var numMembers int
		var maxOnline int
		var joins, leaves int
		var maxVoice int

		err = rows.Scan(&t, &numMembers, &maxOnline, &joins, &leaves, &maxVoice)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		sum.JoinedDay += joins
		sum.LeftDay += leaves
		if maxOnline > sum.Online {
			sum.Online = maxOnline
		}

		if numMembers > sum.TotalMembers {
			sum.TotalMembers = numMembers
		}
	}

	return sum, nil
}

type ChartDataPeriod struct {
	T          time.Time `json:"t"`
	Joins      int       `json:"joins"`
	Leaves     int       `json:"leaves"`
	NumMembers int       `json:"num_members"`
	MaxOnline  int       `json:"max_online"`
	Messages   int       `json:"num_messages"`
}

func RetrieveChartDataPeriods(ctx context.Context, guildID int64, t time.Time, days int) ([]*ChartDataPeriod, error) {
	const q = `SELECT t, num_messages, num_members, max_online, joins, leaves, max_voice
	FROM server_stats_periods_compressed
	WHERE t > $2 AND guild_id = $1 and t < $3
	ORDER BY t DESC;`

	if days <= 0 {
		days = 1000
	}

	rows, err := common.PQ.QueryContext(ctx, q, guildID, t.Add(time.Hour*-24*time.Duration(days+1)), t.Truncate(time.Hour*24))
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	defer rows.Close()

	periods := make([]*ChartDataPeriod, 0, days)
	for rows.Next() {
		var t time.Time
		var numMessages, numMembers, maxOnline, joins, leaves, maxVoice int

		err = rows.Scan(&t, &numMessages, &numMembers, &maxOnline, &joins, &leaves, &maxVoice)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		periods = append(periods, &ChartDataPeriod{
			T:          t,
			Joins:      joins,
			Leaves:     leaves,
			NumMembers: numMembers,
			MaxOnline:  maxOnline,
			Messages:   numMessages,
		})
	}

	return periods, nil
}
