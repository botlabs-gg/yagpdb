package feeds

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"strings"
	"sync"
)

type Plugin interface {
	common.Plugin

	StartFeed()
	StopFeed(*sync.WaitGroup)
}

var (
	runningPlugins = make([]Plugin, 0)
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
				logrus.Info("Ignoring feed", plugin.PluginInfo().Name)
				continue
			}
		}

		logrus.Info("Starting feed ", plugin.PluginInfo().Name)
		go fp.StartFeed()
		runningPlugins = append(runningPlugins, fp)
	}
}

func Stop(wg *sync.WaitGroup) {
	for _, plugin := range runningPlugins {
		logrus.Info("Stopping feed ", plugin.PluginInfo().Name)
		wg.Add(1)
		go plugin.StopFeed(wg)
	}
}
