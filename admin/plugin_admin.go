package admin

import (
	"github.com/RhykerWells/yagpdb/v2/common"
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct {
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Admin",
		SysName:  "admin",
		Category: common.PluginCategoryCore,
	}
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
