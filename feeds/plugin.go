package feeds

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
)

type Plugin interface {
	StartFeed()
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

func Run() {
	for _, plugin := range Plugins {
		logrus.Info("Starting feed ", plugin.Name())
		go plugin.StartFeed()
	}
}
