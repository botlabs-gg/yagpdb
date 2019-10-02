package common

var (
	Plugins []Plugin
)

type PluginCategory struct {
	Name  string
	Order int
}

var (
	PluginCategoryCore       = &PluginCategory{Name: "Core", Order: 0}
	PluginCategoryModeration = &PluginCategory{Name: "Moderation", Order: 10}
	PluginCategoryMisc       = &PluginCategory{Name: "Misc", Order: 20}
	PluginCategoryFeeds      = &PluginCategory{Name: "Feeds", Order: 30}
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

// RegisterPlugin registers a plugin, should be called when the bot is starting up
func RegisterPlugin(plugin Plugin) {
	Plugins = append(Plugins, plugin)
	logger.Info("Registered plugin: " + plugin.PluginInfo().Name)
}
