package pqkeydb

import (
	"fmt"
	"os"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common/testutils"
)

var db *DB

func TestMain(m *testing.M) {
	conn, err := testutils.InitPQ([]string{"keydb"}, DBSchemas)
	if err != nil {
		fmt.Println("Failed connecting to postgres database, not running tests: ", err)
		return
	}

	db = &DB{
		PQ: conn,
	}

	os.Exit(m.Run())
}

func TestGetSet(t *testing.T) {
	key := "tkey1"
	value := "some string"

	_, err := db.SetString(0, key, value)
	if err != nil {
		t.Error(err)
		return
	}

	v, err := db.Get(0, key).Str()
	if err != nil {
		t.Error(err)
		return
	}

	if v != value {
		t.Errorf("Extected %q, got %q", value, v)
	}

	// make sure this value was only set on guilid 0
	_, err = db.Get(1, key).Str()
	if err != ErrKeyNotFound {
		t.Error("Expected ErrKeyNotFound")
	}

}
