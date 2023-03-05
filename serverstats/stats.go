package serverstats

import (
	"context"
	"strconv"
	"time"

	"emperror.dev/errors"

	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/serverstats/messagestatscollector"
	"github.com/mediocregopher/radix/v3"
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

	msgStats, err := readDailyMsgStats(t, guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	miscStats, err := readDailyMiscStats(t, guildID)
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

func readDailyMsgStats(t time.Time, guildID int64) (channelStats map[string]*ChannelStats, err error) {
	day := t.YearDay()
	year := t.Year()

	raw := make(map[int64]int64)
	err = common.RedisPool.Do(radix.Cmd(&raw, "ZRANGE", messagestatscollector.KeyMessageStats(guildID, year, day), "0", "-1", "WITHSCORES"))
	if err != nil {
		return nil, err
	}

	channelStats = make(map[string]*ChannelStats)
	for k, v := range raw {
		strID := strconv.FormatInt(k, 10)
		channelStats[strID] = &ChannelStats{
			Name:  strID,
			Count: v,
		}
	}

	return channelStats, err
}

func readDailyMiscStats(t time.Time, guildID int64) (*DailyStats, error) {
	// read the rest

	year := t.Year()
	day := t.YearDay()

	var totalMembers int
	var joins int
	var leaves int
	err := common.RedisPool.Do(radix.Pipeline(
		radix.FlatCmd(&totalMembers, "ZSCORE", keyTotalMembers(year, day), guildID),
		radix.FlatCmd(&joins, "ZSCORE", keyJoinedMembers(year, day), guildID),
		radix.FlatCmd(&leaves, "ZSCORE", keyLeftMembers(year, day), guildID),
	))

	ds := &DailyStats{
		TotalMembers: totalMembers,
		JoinedDay:    joins,
		LeftDay:      leaves,
	}

	return ds, err
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
