package rsvp

//go:generate sqlboiler --no-hooks psql

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/when"
	"github.com/botlabs-gg/yagpdb/v2/lib/when/rules"
	wcommon "github.com/botlabs-gg/yagpdb/v2/lib/when/rules/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/when/rules/en"
	"github.com/botlabs-gg/yagpdb/v2/timezonecompanion/trules"
)

var (
	logger = common.GetPluginLogger(&Plugin{})

	dateParser *when.Parser
)

const (
	EventAccepted  = "event_accepted"
	EventRejected  = "event_rejected"
	EventWaitlist  = "event_waitlist"
	EventUndecided = "event_undecided"
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
