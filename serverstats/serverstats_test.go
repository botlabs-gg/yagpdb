package serverstats

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"

	"github.com/botlabs-gg/yagpdb/v2/common/testutils"
)

var db *sql.DB

func TestMain(m *testing.M) {
	conn, err := testutils.InitPQ([]string{"server_stats_hourly_periods_messages", "server_stats_hourly_periods_misc", "server_stats_periods_compressed", "server_stats_periods", "server_stats_member_periods"}, append(legacyDBSchemas, dbSchemas...))
	if err != nil {
		fmt.Println("Failed connecting to postgres database, not running tests: ", err)
		return
	}

	db = conn
	common.PQ = db

	os.Exit(m.Run())
}
