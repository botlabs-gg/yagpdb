package serverstats

import (
	"context"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/testutils"
)

func TestCompressStats(t *testing.T) {
	defer testutils.ClearTables(db, "server_stats_hourly_periods_messages", "server_stats_hourly_periods_misc", "server_stats_periods_compressed")

	// 10th dec 10:10:0
	tim := time.Date(2019, time.December, 10, 10, 10, 0, 0, time.UTC)
	// insert mock rows into temp tables, testing bounds

	InsertMessageRow(1, 2, tim, 5)                    // this row should be ignored since its in the current day
	InsertMessageRow(1, 2, tim.Add(time.Hour*-40), 5) // this row should be ignored since its in the current day

	InsertMessageRow(1, 2, tim.Add(time.Hour*-23), 10)                                      // should be included
	InsertMessageRow(1, 2, tim.Add((time.Hour*-23)+(time.Minute*59)+(time.Second*59)), 10)  // included
	InsertMessageRow(1, 2, tim.Add((time.Hour*-24)+(time.Minute*59)+(time.Second*59)), 100) // included

	InsertMessageRow(1, 2, tim.Add(time.Hour*-25), 10) // also included

	/////////////////////

	InsertMiscHourlyRow(1, tim, 50, 10, 5, 20) // this row should be ignored in the daily call since its the same hour

	InsertMiscHourlyRow(1, tim.Add(time.Hour*-40), 50, 10, 5, 20)                                     // should be included
	InsertMiscHourlyRow(1, tim.Add(time.Hour*-23), 50, 10, 5, 20)                                     // should be included
	InsertMiscHourlyRow(1, tim.Add((time.Hour*-23)+(time.Minute*59)+(time.Second*59)), 55, 10, 5, 25) // included
	InsertMiscHourlyRow(1, tim.Add((time.Hour*-24)+(time.Minute*59)+(time.Second*59)), 60, 10, 5, 20) // included

	InsertMiscHourlyRow(1, tim.Add(time.Hour*-25), 70, 10, 5, 20) // also included

	// run compression, and maake sure it ran
	compressor := &Compressor{}
	ran, next, err := compressor.updateCompress(tim, true)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	if !ran {
		t.Error("Did not run")
		return
	}

	expected := time.Date(2019, time.December, 11, 1, 0, 0, 0, time.UTC).Sub(tim)
	// expected :=
	if next != expected {
		t.Errorf("Next is %s, expected %s", next, expected)
		return
	}

	// make sure proper rows were compressed
	periods, err := RetrieveChartDataPeriods(context.Background(), 1, tim, 7)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	if len(periods) != 2 {
		t.Errorf("Got len of %d, expected 2", len(periods))
	}

	for _, v := range periods {

		t.Logf("Row: %#v", v)

		if tim.Truncate(time.Hour*24) == v.T {
			t.Error("Got same day stats row?")
		}
	}
}

func TestCleanupStats(t *testing.T) {

}
