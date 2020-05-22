package analytics

import (
	"sync"

	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/mediocregopher/radix/v3"
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct {
	stopWorkers chan *sync.WaitGroup
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "analytics",
		SysName:  "analytics",
		Category: common.PluginCategoryCore,
	}
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{
		stopWorkers: make(chan *sync.WaitGroup),
	})
	common.InitSchemas("analytics", dbSchemas...)
}

func RecordActiveUnit(guildID int64, plugin common.Plugin, analyticName string) {
	err := recordActiveUnit(guildID, plugin, analyticName)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("plugin", plugin.PluginInfo().SysName).WithField("analytic", analyticName).Error("Failed updating analytic in redis")
	}
}

var confEnableAnalytics = config.RegisterOption("yagpdb.enable_analytics", "Enable usage analytics tracking", false)

func recordActiveUnit(guildID int64, plugin common.Plugin, analyticName string) error {
	if !confEnableAnalytics.GetBool() {
		return nil
	}

	err := common.RedisPool.Do(radix.FlatCmd(nil, "HINCRBY", "anaylytics_active_units."+plugin.PluginInfo().SysName+"."+analyticName, guildID, 1))
	if err != nil {
		return err
	}

	// logger.Debug("Incrementing analytic " + plugin.PluginInfo().SysName + "." + analyticName)

	return nil
}
