package common

import (
	"github.com/sirupsen/logrus"
)

var (
	Plugins []Plugin
)

type PluginCategory struct {
	Name string
}

var (
	PluginCategoryCore       = &PluginCategory{Name: "Core"}
	PluginCategoryModeration = &PluginCategory{Name: "Moderation"}
	PluginCategoryFeeds      = &PluginCategory{Name: "Feeds"}
	PluginCategoryMisc       = &PluginCategory{Name: "Misc"}
)

type PluginInfo struct {
	Name     string // Human readable name of the plugin
	SysName  string // snake_case version of the name in lower case
	Category *PluginCategory
}

// Plugin represents a plugin, all plugins needs to implement this at a bare minimum
type Plugin interface {
	PluginInfo() *PluginInfo
}

type PluginWithLogging interface {
	Logger() *logrus.Entry
	SetLogger(entry *logrus.Entry)
}

// RegisterPlugin registers a plugin, should be called when the bot is starting up
func RegisterPlugin(plugin Plugin) {
	Plugins = append(Plugins, plugin)
	logrus.Info("Registered plugin: " + plugin.PluginInfo().Name)
}
