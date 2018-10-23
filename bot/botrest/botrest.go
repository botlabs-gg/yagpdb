package botrest

import (
	"strconv"
)

func RedisKeyShardAddressMapping(shardID int) string {
	return "botrest_shard_mapping:" + strconv.Itoa(shardID)
}
