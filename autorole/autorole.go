package autorole

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
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

func GetGeneralConfig(guildID int64) (*GeneralConfig, error) {
	conf := &GeneralConfig{}
	err := common.GetRedisJson(KeyGeneral(guildID), conf)
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("failed retreiving autorole config")
	}
	return conf, err
}
