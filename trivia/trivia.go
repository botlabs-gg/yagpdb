package trivia

import "github.com/botlabs-gg/yagpdb/v2/common"

type Plugin struct {
	stopCleanup chan struct{}
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Trivia",
		SysName:  "trivia",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.InitSchemas("trivia", DBSchemas...)
	common.RegisterPlugin(&Plugin{
		stopCleanup: make(chan struct{}),
	})
}
