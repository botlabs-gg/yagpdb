package feeds

import (
	"strings"
	"sync"

	"github.com/jonas747/yagpdb/common"
)

type Plugin interface {
	common.Plugin

	StartFeed()
	StopFeed(*sync.WaitGroup)
}

var (
	runningPlugins = make([]Plugin, 0)
	logger         = common.GetFixedPrefixLogger("feeds")
)

// Run runs the specified feeds
func Run(which []string) {
	for _, plugin := range common.Plugins {
		fp, ok := plugin.(Plugin)
		if !ok {
			continue
		}

		if len(which) > 0 {
			found := false

			for _, feed := range which {
				if strings.EqualFold(feed, plugin.PluginInfo().Name) {
					found = true
					break
				}
			}

			if !found {
				logger.Info("Ignoring feed", plugin.PluginInfo().Name)
				continue
			}
		}

		logger.Info("Starting feed ", plugin.PluginInfo().Name)
		go fp.StartFeed()
		runningPlugins = append(runningPlugins, fp)
	}
}

func Stop(wg *sync.WaitGroup) {
	for _, plugin := range runningPlugins {
		logger.Info("Stopping feed ", plugin.PluginInfo().Name)
		wg.Add(1)
		go plugin.StopFeed(wg)
	}
}
