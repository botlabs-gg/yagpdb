package autorole

import (
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var configCache = common.CacheSet.RegisterSlot("autorole_config", func(key interface{}) (interface{}, error) {
	config, err := GetGeneralConfig(key.(int64))
	return config, err
}, int64(0))

var logger = common.GetPluginLogger(&Plugin{})

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
	RequiredDuration int   `valid:"0,"`

	RequiredRoles            []int64 `valid:"role,true"`
	IgnoreRoles              []int64 `valid:"role,true"`
	OnlyOnJoin               bool
	AssignRoleAfterScreening bool
}

const (
	FullScanStarted int = iota + 1
	FullScanIterating
	FullScanIterationDone
	FullScanAssigningRole
	FullScanCancelled
)

func GetGeneralConfig(guildID int64) (*GeneralConfig, error) {
	conf := &GeneralConfig{}
	err := common.GetRedisJson(KeyGeneral(guildID), conf)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed retreiving autorole config")
	}
	return conf, err
}
