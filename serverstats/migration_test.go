package serverstats

import (
	"context"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/testutils"
)

func TestMigrationToV2FormatMsgs(t *testing.T) {
	defer testutils.ClearTables(db, "server_stats_periods", "server_stats_periods_compressed")

	// 10th dec 10:10:0
	tim := time.Date(2019, time.December, 10, 10, 10, 0, 0, time.UTC)

	insertLegacyMsgRow(1, tim, 10, 20)
	insertLegacyMsgRow(1, tim.Add(time.Hour), 10, 20)
	insertLegacyMsgRow(1, tim.Add(time.Hour*2), 10, 20)

	tim = tim.Add(time.Hour * 24)

	insertLegacyMsgRow(1, tim, 10, 20)
	insertLegacyMsgRow(1, tim.Add(time.Hour), 10, 20)
	insertLegacyMsgRow(1, tim.Add(time.Hour*2), 10, 50)

	newLast, err := migrateChunkV2Messages(-1, nil)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	if newLast != 6 {
		t.Errorf("LastID is %d, expected %d", newLast, 6)
		return
	}

	chartData, err := RetrieveChartDataPeriods(context.Background(), 1, tim.Add(time.Hour*48), 3)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	if len(chartData) != 2 {
		t.Errorf("got unexpected length: %d, but expected %d", len(chartData), 2)
		return
	}
}

func insertLegacyMsgRow(guildID int64, tim time.Time, cID int64, count int) {
	const q = `INSERT INTO server_stats_periods (started, duration, guild_id, user_id, channel_id, count)
	VALUES                                        ($1,      0,         $2,     0,         $3,       $4) 
	`

	_, err := db.Exec(q, tim, guildID, cID, count)
	if err != nil {
		panic(err)
	}
}
