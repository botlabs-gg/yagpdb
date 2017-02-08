package feeds

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"strings"
	"sync"
)

type Plugin interface {
	StartFeed()
	StopFeed(*sync.WaitGroup)
	Name() string
}

var (
	Plugins []Plugin
)

// Register a plugin
func RegisterPlugin(plugin Plugin) {
	if Plugins == nil {
		Plugins = []Plugin{plugin}
	} else {
		Plugins = append(Plugins, plugin)
	}

	common.AddPlugin(plugin)
}

// Run runs the specified feeds
func Run(which []string) {
	for _, plugin := range Plugins {
		if len(which) > 0 {
			found := false

			for _, feed := range which {
				if strings.EqualFold(feed, plugin.Name()) {
					found = true
					break
				}
			}

			if !found {
				logrus.Info("Ignoring feed", plugin.Name())
				continue
			}
		}

		logrus.Info("Starting feed ", plugin.Name())
		go plugin.StartFeed()
	}
}

func Stop(wg *sync.WaitGroup) {
	for _, plugin := range Plugins {
		logrus.Info("Stopping feed ", plugin.Name())
		wg.Add(1)
		go plugin.StopFeed(wg)
	}
}
