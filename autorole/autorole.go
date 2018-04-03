package autorole

//go:generate esc -o assets_gen.go -pkg autorole -ignore ".go" assets/

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"strconv"
)

func KeyGeneral(guildID int64) string { return "autorole:" + discordgo.StrID(guildID) + ":general" }
func KeyProcessing(guildID int64) string {
	return "autorole:" + discordgo.StrID(guildID) + ":processing"
}

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Autorole"
}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)
}

type GeneralConfig struct {
	Role             int64 `json:",string" valid:"role,true"`
	RequiredDuration int

	RequiredRoles []int64 `valid:"role,true"`
	IgnoreRoles   []int64 `valid:"role,true"`
	OnlyOnJoin    bool
}

type LegacyGeneralConfig struct {
	Role             string `valid:"role,true"`
	RequiredDuration int

	RequiredRoles []int64 `valid:"role,true"`
	IgnoreRoles   []int64 `valid:"role,true"`
	OnlyOnJoin    bool
}

func (l *GeneralConfig) UnmarshalJSON(b []byte) error {
	var tmp LegacyGeneralConfig
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}

	l.Role, _ = strconv.ParseInt(tmp.Role, 10, 64)
	l.RequiredDuration = tmp.RequiredDuration
	l.RequiredRoles = tmp.RequiredRoles
	l.IgnoreRoles = tmp.IgnoreRoles
	l.OnlyOnJoin = tmp.OnlyOnJoin

	return nil
}

func GetGeneralConfig(client *redis.Client, guildID int64) (*GeneralConfig, error) {
	conf := &GeneralConfig{}
	err := common.GetRedisJson(client, KeyGeneral(guildID), conf)
	return conf, err
}
