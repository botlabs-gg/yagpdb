package common

import (
	"github.com/sirupsen/logrus"
	"sync"
)

var (
	Plugins []Plugin
)

// Plugin represents a plugin, all plugins needs to implement this at a bare minimum
type Plugin interface {
	Name() string
}

type PluginWithLogging interface {
	Logger() *logrus.Entry
	SetLogger(entry *logrus.Entry)
}

// RegisterPlugin registers a plugin, should be called when the bot is starting up
func RegisterPlugin(plugin Plugin) {
	Plugins = append(Plugins, plugin)
	if cast, ok := plugin.(PluginWithLogging); ok {
		cast.SetLogger(logrus.WithField("P", plugin.Name()))
	}
}

// RegisterPluginL registers a plugin, should be called when the bot is starting up
func RegisterPluginL(pl Plugin) {
	if _, ok := pl.(PluginWithLogging); !ok {
		logrus.Fatal("Not a PluginWithLogging: ", pl.Name())
	}

	RegisterPlugin(pl)
}

type BasePlugin struct {
	Entry *logrus.Entry
}

var _ PluginWithLogging = (*BasePlugin)(nil)

func (p *BasePlugin) Logger() *logrus.Entry {
	return p.Entry
}

func (p *BasePlugin) SetLogger(entry *logrus.Entry) {
	p.Entry = entry
}

type BackgroundWorkerPlugin interface {
	RunBackgroundWorker()
	StopBackgroundWorker(wg *sync.WaitGroup)
}

func RunBackgroundWorkers() {
	for _, p := range Plugins {
		if bwc, ok := p.(BackgroundWorkerPlugin); ok {
			logrus.Info("Running background worker: ", p.Name())
			go bwc.RunBackgroundWorker()
		}
	}
}

func StopBackgroundWorkers(wg *sync.WaitGroup) {
	for _, p := range Plugins {
		if bwc, ok := p.(BackgroundWorkerPlugin); ok {
			logrus.Info("Stopping background worker: ", p.Name())
			wg.Add(1)
			go bwc.StopBackgroundWorker(wg)
		}
	}
}
