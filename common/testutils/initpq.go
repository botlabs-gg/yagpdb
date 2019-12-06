package testutils

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	// postgres driver
	_ "github.com/lib/pq"
)

// ConnectPQ connectes to a postgres database for testing purposes
func ConnectPQ() (*sql.DB, error) {
	host := os.Getenv("YAGPDB_TEST_PQ_HOST")
	if host == "" {
		host = "localhost"
	}
	user := os.Getenv("YAGPDB_TEST_PQ_USER")
	if user == "" {
		user = "yagpdb_test"
	}

	dbPassword := os.Getenv("YAGPDB_TEST_PQ_PASSWORD")
	sslMode := os.Getenv("YAGPDB_TEST_PQ_SSLMODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	dbName := os.Getenv("YAGPDB_TEST_PQ_DB")
	if dbName == "" {
		dbName = "yagpdb_test"
	}

	if !strings.Contains(dbName, "test") {
		panic("Test database name has to contain 'test'T this is a safety measure to protect against running tests on production systems.")
	}

	connStr := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s password='%s'", host, user, dbName, sslMode, dbPassword)
	connStrPWCensored := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s password='%s'", host, user, dbName, sslMode, "***")
	fmt.Println("Postgres connection string being used: " + connStrPWCensored)

	conn, err := sql.Open("postgres", connStr)
	return conn, err
}

// InitTables will drop the provided tables and initialize the new ones
func InitTables(db *sql.DB, dropTables []string, initQueries []string) error {
	for _, v := range dropTables {
		_, err := db.Exec("DROP TABLE IF EXISTS " + v)
		if err != nil {
			return err
		}
	}

	for _, v := range initQueries {
		_, err := db.Exec(v)
		if err != nil {
			return err
		}
	}

	return nil
}

// InitPQ is a helper that calls both ConnectPQ and InitTables
func InitPQ(dropTables []string, initQueries []string) (*sql.DB, error) {
	db, err := ConnectPQ()
	if err != nil {
		return nil, err
	}

	err = InitTables(db, dropTables, initQueries)
	return db, err
}

// ClearTables deletes all rows from a table, and panics if an error occurs
// usefull for defers for test cleanup
func ClearTables(db *sql.DB, tables ...string) {
	for _, v := range tables {
		_, err := db.Exec("DELETE FROM " + v + ";")
		if err != nil {
			panic(err)
		}
	}
}
