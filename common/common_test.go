package common

import (
	"fmt"
	"os"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common/testutils"
)

func TestMain(m *testing.M) {
	conn, err := testutils.InitPQ([]string{"local_incr_ids"}, []string{localIDsSchema})
	if err != nil {
		fmt.Println("Failed connecting to postgres database, not running tests: ", err)
		return
	}

	PQ = conn

	os.Exit(m.Run())
}
