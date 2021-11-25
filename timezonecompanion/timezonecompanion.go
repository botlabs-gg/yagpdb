package timezonecompanion

//go:generate sqlboiler --no-hooks psql
//go:generate go run generate/generatemappings.go

import (
	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/timezonecompanion/trules"
	"github.com/jonas747/when"
	"github.com/jonas747/when/rules"
)

type Plugin struct {
	DateParser *when.Parser
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "TimezoneCompanion",
		SysName:  "timezonecompanion",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {

	w := when.New(&rules.Options{
		Distance:     10,
		MatchByOrder: true})

	w.Add(trules.Hour(rules.Override), trules.HourMinute(rules.Override))

	common.InitSchemas("timezonecompanion", DBSchemas...)
	common.RegisterPlugin(&Plugin{
		DateParser: w,
	})
}
