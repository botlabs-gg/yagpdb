// Package pqkeydb is a simple key-value database on top of postgres
package pqkeydb

import (
	"database/sql"
	"strconv"
	"time"

	"emperror.dev/errors"
)

// DBSchemas is a slice of commands that should be ran to initialize the database
var DBSchemas = []string{
	`
CREATE TABLE IF NOT EXISTS keydb (
	guild_id BIGINT NOT NULL,
	key TEXT NOT NULL,
	new BOOLEAN NOT NULl,

	value TEXT NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	PRIMARY KEY (guild_id, key)
)
	`,
}

// DB represents a database
type DB struct {
	PQ *sql.DB
}

// SetString creates a new or updates an existing string in the database
func (db *DB) SetString(guildID int64, key, value string) (new bool, err error) {
	const q = `INSERT INTO keydb (guild_id, key, value, updated_at, new) 
	VALUES ($1, $2, $3, now(), true)
	ON CONFLICT (guild_id, key) 
	DO UPDATE SET value = $3, new = false, updated_at = now()
	RETURNING keydb.new;`

	err = db.PQ.QueryRow(q, guildID, key, value).Scan(&new)
	return
}

// ErrKeyNotFound is returned when the key is not found
var ErrKeyNotFound = errors.NewPlain("Key not found")

// Get returns the entry at key for the specified guild
func (db *DB) Get(guildID int64, key string) (r *Result) {
	const q = `SELECT guild_id, key, value, updated_at FROM keydb WHERE guild_id=$1 AND key=$2;`

	entry := Entry{}

	err := db.PQ.QueryRow(q, guildID, key).Scan(&entry.GuildID, &entry.Key, &entry.Value, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		err = ErrKeyNotFound
	}

	return &Result{
		Error: err,
		Entry: entry,
	}
}

// Result represents a result form the Get operations
type Result struct {
	Entry Entry

	// Set if an error occured
	Error error
}

// Entry represents a entry in the keydb
type Entry struct {
	GuildID int64
	Key     string

	Value     string
	UpdatedAt time.Time
}

// Str returns the string value of the entry, or an error if an error occured
func (r *Result) Str() (string, error) {
	return r.Entry.Value, r.Error
}

// Int64 returns the value parsed as a int64, or an error if an error occured
func (r *Result) Int64() (int64, error) {
	if r.Error != nil {
		return 0, r.Error
	}

	return strconv.ParseInt(r.Entry.Value, 10, 64)
}
