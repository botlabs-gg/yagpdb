package bot

type Plugin interface {
	// Called when the plugin is supposed to be initialized
	// That is add comnands, discord event handlers
	InitBot()
	Name() string
}

var plugins []Plugin

// Register a plugin, should only be called before webserver is started!!!
func RegisterPlugin(plugin Plugin) {
	if plugins == nil {
		plugins = []Plugin{plugin}
	} else {
		plugins = append(plugins, plugin)
	}
}
