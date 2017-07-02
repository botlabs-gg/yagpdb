package autorole

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
)

func KeyCommands(guildID string) string   { return "autorole:" + guildID + ":commands" }
func KeyGeneral(guildID string) string    { return "autorole:" + guildID + ":general" }
func KeyProcessing(guildID string) string { return "autorole:" + guildID + ":processing" }

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Autorole"
}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)
}

type RoleCommand struct {
	Role string
	Name string
}

type GeneralConfig struct {
	Role             string
	RequiredDuration int
}

func GetGeneralConfig(client *redis.Client, guildID string) (*GeneralConfig, error) {
	conf := &GeneralConfig{}
	err := common.GetRedisJson(client, KeyGeneral(guildID), conf)
	return conf, err
}

func GetCommands(client *redis.Client, guildID string) (roles []*RoleCommand, err error) {
	err = common.GetRedisJson(client, KeyCommands(guildID), &roles)
	return
}
