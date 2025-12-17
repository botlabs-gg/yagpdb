package trivia

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

type Plugin struct {
	stopWorkers chan *sync.WaitGroup
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
		stopWorkers: make(chan *sync.WaitGroup),
	})
}
