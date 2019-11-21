package admin

import (
	"github.com/jonas747/yagpdb/common"
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
