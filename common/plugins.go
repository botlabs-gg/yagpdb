package common

import (
	"github.com/sirupsen/logrus"
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
	} else {
		logrus.Warn(plugin.Name(), " is not a PluginWithLogging")
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
