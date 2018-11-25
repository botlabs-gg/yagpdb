package botrest

import (
	"strconv"
)

func RedisKeyShardAddressMapping(shardID int) string {
	return "botrest_shard_mapping:" + strconv.Itoa(shardID)
}

func RedisKeyNodeAddressMapping(nodeID string) string {
	return "botrest_node_mapping:" + nodeID
}
