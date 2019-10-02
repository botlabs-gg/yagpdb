package common

import (
	"strconv"

	"github.com/jonas747/retryableredis"
)

func GenLocalIncrID(guildID int64, key string) (int64, error) {
	var id int64
	err := RedisPool.Do(retryableredis.Cmd(&id, "HINCRBY", "local_ids:"+strconv.FormatInt(guildID, 10), key, "1"))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("key", key).Error("failed incrementing local id")
	}

	return id, err
}
