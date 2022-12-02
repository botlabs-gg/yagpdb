package serverstats

import (
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/testutils"
)

func TestDailyStatsMessages(t *testing.T) {
	tim := time.Now()

	defer testutils.ClearTables(db, "server_stats_hourly_periods_messages")

	InsertMessageRow(1, 2, tim, 5) // this row should be ignored in the daily call since its the same hour

	InsertMessageRow(1, 2, tim.Add(time.Hour*-23), 10)                                      // should be included
	InsertMessageRow(1, 2, tim.Add((time.Hour*-23)+(time.Minute*59)+(time.Second*59)), 10)  // included
	InsertMessageRow(1, 2, tim.Add((time.Hour*-24)+(time.Minute*59)+(time.Second*59)), 100) // included

	InsertMessageRow(1, 2, tim.Add(time.Hour*-25), 10) // out of range

	daily, err := RetrieveDailyStats(tim, 1)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	if daily.ChannelMessages["2"].Count != 120 {
		t.Errorf("Incorrect count, got %d expected %d", daily.ChannelMessages["2"].Count, 120)
	}
}

func InsertMessageRow(gID int64, cID int64, t time.Time, count int) {
	t = RoundHour(t)

	const updateQuery = `
	INSERT INTO server_stats_hourly_periods_messages (guild_id, t, channel_id, count) 
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (guild_id, channel_id, t) DO UPDATE
	SET count = server_stats_hourly_periods_messages.count + $4`

	_, err := db.Exec(updateQuery, gID, t, cID, count)
	if err != nil {
		panic(err)
	}
}

func TestDailyMisc(t *testing.T) {
	tim := time.Now()

	defer testutils.ClearTables(db, "server_stats_hourly_periods_misc")

	InsertMiscHourlyRow(1, tim, 50, 10, 5, 20) // this row should be ignored in the daily call since its the same hour

	InsertMiscHourlyRow(1, tim.Add(time.Hour*-23), 50, 10, 5, 20)                                     // should be included
	InsertMiscHourlyRow(1, tim.Add((time.Hour*-23)+(time.Minute*59)+(time.Second*59)), 55, 10, 5, 25) // included
	InsertMiscHourlyRow(1, tim.Add((time.Hour*-24)+(time.Minute*59)+(time.Second*59)), 60, 10, 5, 20) // included

	InsertMiscHourlyRow(1, tim.Add(time.Hour*-25), 70, 10, 5, 20) // out of range

	daily, err := RetrieveDailyStats(tim, 1)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	if daily.JoinedDay != 30 {
		t.Errorf("join: expected %d, got %d", 30, daily.JoinedDay)
	}
	if daily.LeftDay != 15 {
		t.Errorf("leaves: expected %d, got %d", 15, daily.LeftDay)
	}
	if daily.TotalMembers != 60 {
		t.Errorf("total: expected %d, got %d", 60, daily.TotalMembers)
	}
	if daily.Online != 25 {
		t.Errorf("total: expected %d, got %d", 25, daily.Online)
	}

}

func InsertMiscHourlyRow(gID int64, t time.Time, numMembers, joins, leaves, maxOnline int) {
	t = RoundHour(t)

	const q = `INSERT INTO server_stats_hourly_periods_misc  (guild_id, t, num_members, joins, leaves, max_online, max_voice)
	VALUES ($1, $2, $3, $4, $5, $6, 0)
	ON CONFLICT (guild_id, t)
	DO UPDATE SET 
	max_online = GREATEST (server_stats_hourly_periods_misc.max_online, $6),
	joins = server_stats_hourly_periods_misc.joins + $4,
	leaves = server_stats_hourly_periods_misc.leaves + $5,
	num_members = GREATEST (server_stats_hourly_periods_misc.num_members, $3)`

	_, err := db.Exec(q, gID, t, numMembers, joins, leaves, maxOnline)
	if err != nil {
		panic(err)
	}
}
