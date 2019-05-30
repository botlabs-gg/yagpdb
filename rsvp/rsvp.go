package rsvp

//go:generate sqlboiler --no-hooks psql

import (
	"github.com/jonas747/yagpdb/common"
	"sync"
)

var (
	logger = common.GetPluginLogger(&Plugin{})
)

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

	common.RegisterPlugin(p)
}
