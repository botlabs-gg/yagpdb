package autorole

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
)

var confDisableNonPremiumRetroActiveAssignment = config.RegisterOption("yagpdb.autorole.non_premium_retroactive_assignment", "Wether to enable retroactive assignemnt on non premium guilds", true)

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
	RequiredDuration int

	RequiredRoles []int64 `valid:"role,true"`
	IgnoreRoles   []int64 `valid:"role,true"`
	OnlyOnJoin    bool
}

func GetGeneralConfig(guildID int64) (*GeneralConfig, error) {
	conf := &GeneralConfig{}
	err := common.GetRedisJson(KeyGeneral(guildID), conf)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed retreiving autorole config")
	}
	return conf, err
}
