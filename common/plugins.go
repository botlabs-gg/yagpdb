package common

var (
	Plugins []Plugin
)

// Plugin represents a plugin, all plugins needs to implement this at a bare minimum
type Plugin interface {
	Name() string
}

// RegisterPlugin registers a plugin, should be called when the bot is starting up
func RegisterPlugin(plugin Plugin) {
	Plugins = append(Plugins, plugin)
}
