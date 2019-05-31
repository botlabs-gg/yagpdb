package timezonecompanion

//go:generate sqlboiler --no-hooks psql
//go:generate go run generate/generatemappings.go

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules"
	"github.com/olebedev/when/rules/en"
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

	w.Add(en.Hour(rules.Override), en.HourMinute(rules.Override))

	common.InitSchema(DBSchema, "timezonecompanion")
	common.RegisterPlugin(&Plugin{
		DateParser: w,
	})
}
