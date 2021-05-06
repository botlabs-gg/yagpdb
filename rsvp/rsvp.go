package rsvp

//go:generate sqlboiler --no-hooks psql

import (
	"sync"

	"github.com/jonas747/when"
	"github.com/jonas747/when/rules"
	wcommon "github.com/jonas747/when/rules/common"
	"github.com/jonas747/when/rules/en"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/timezonecompanion/trules"
)

var (
	logger = common.GetPluginLogger(&Plugin{})

	dateParser *when.Parser
)

func init() {
	dateParser = when.New(&rules.Options{
		Distance:     10,
		MatchByOrder: true})

	dateParser.Add(
		en.Weekday(rules.Override),
		en.CasualDate(rules.Override),
		en.CasualTime(rules.Override),
		trules.Hour(rules.Override),
		trules.HourMinute(rules.Override),
		en.Deadline(rules.Override),
		en.ExactMonthDate(rules.Override),
	)
	dateParser.Add(wcommon.All...)
}

type Plugin struct {
	setupSessions   []*SetupSession
	setupSessionsMU sync.Mutex
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "RSVP",
		SysName:  "rsvp",
		Category: common.PluginCategoryMisc,
	}
}

func RegisterPlugin() {
	p := &Plugin{}

	common.InitSchemas("rsvp", DBSchemas...)
	common.RegisterPlugin(p)
}
