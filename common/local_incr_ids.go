package common

import (
	"github.com/mediocregopher/radix"
	"github.com/sirupsen/logrus"
	"strconv"
)

func GenLocalIncrID(guildID int64, key string) (int64, error) {
	var id int64
	err := RedisPool.Do(radix.Cmd(&id, "HINCRBY", "local_ids:"+strconv.FormatInt(guildID, 10), key, "1"))
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).WithField("key", key).Error("failed incrementing local id")
	}

	return id, err
}
