package common

import (
	"database/sql"
	"strconv"

	"emperror.dev/errors"
	"github.com/mediocregopher/radix/v3"
)

// GenLocalIncrID creates a new or incremements a existing local id incrememter
// used to have per guild id's
//
// GenLocalIncrID is deprecated and GenLocalIncrIDPQ should be used instead
func GenLocalIncrID(guildID int64, key string) (int64, error) {
	var id int64
	err := RedisPool.Do(radix.Cmd(&id, "HINCRBY", "local_ids:"+strconv.FormatInt(guildID, 10), key, "1"))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("key", key).Error("failed incrementing local id")
	}

	return id, err
}

const localIDsSchema = `
CREATE TABLE IF NOT EXISTS local_incr_ids (
	guild_id BIGINT NOT NULL,
	key TEXT NOT NULL,

	last BIGINT NOT NULL,
	last_updated TIMESTAMP WITH TIME ZONE NOT NULL,

	PRIMARY KEY(guild_id, key)
)
`

// GenLocalIncrIDPQ creates a new or incremements a existing local id incrememter
// used to have per guild id's
//
// GenLocalIncrIDPQ differs from GenLocalIncrID in that it uses postgres instead of redis
func GenLocalIncrIDPQ(tx *sql.Tx, guildID int64, key string) (int64, error) {
	const query = `INSERT INTO local_incr_ids (guild_id, key, last, last_updated) 
	VALUES ($1, $2, 1, now()) 
	ON CONFLICT (guild_id, key) 
	DO UPDATE SET last = local_incr_ids.last + 1 
	RETURNING last;`

	var row *sql.Row
	if tx == nil {
		row = PQ.QueryRow(query, guildID, key)
	} else {
		row = tx.QueryRow(query, guildID, key)
	}

	var newID int64
	err := row.Scan(&newID)
	if err != nil {
		return 0, errors.WithStackIf(err)
	}

	return newID, nil
}
