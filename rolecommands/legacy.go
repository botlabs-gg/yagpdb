package rolecommands

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
)

func KeyCommands(guildID string) string { return "autorole:" + guildID + ":commands" }

type LegacyRoleCommand struct {
	Role string
	Name string
}

func GetCommands(client *redis.Client, guildID string) (roles []*RoleCommand, err error) {
	err = common.GetRedisJson(client, KeyCommands(guildID), &roles)
	return
}
