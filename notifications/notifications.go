package notifications

import (
	"github.com/RhykerWells/yagpdb/v2/common"
)

//go:generate sqlboiler --no-hooks psql

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)

	common.InitSchemas("notifications", DBSchemas...)
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "General Notifications",
		SysName:  "notifications",
		Category: common.PluginCategoryFeeds,
	}
}
