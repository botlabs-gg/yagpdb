package autorole

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
)

func KeyGeneral(guildID int64) string { return "autorole:" + discordgo.StrID(guildID) + ":general" }
func KeyProcessing(guildID int64) string {
	return "autorole:" + discordgo.StrID(guildID) + ":processing"
}

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Autorole",
		SysName:  "autorole",
		Category: common.PluginCategoryMisc,
	}
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

func GetGeneralConfig(guildID int64) (*GeneralConfig, error) {
	conf := &GeneralConfig{}
	err := common.GetRedisJson(KeyGeneral(guildID), conf)
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("failed retreiving autorole config")
	}
	return conf, err
}
