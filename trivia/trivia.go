package trivia

import "github.com/RhykerWells/yagpdb/v2/common"

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Trivia",
		SysName:  "trivia",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
