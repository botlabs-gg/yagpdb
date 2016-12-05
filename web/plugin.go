package web

import (
	"github.com/jonas747/yagpdb/common"
)

// Plugin represents a web plugin
type Plugin interface {
	// Parse the templates and set up the http routes here
	// mainMuxer is the root and cpmuxer handles the /cp/ route
	// the cpmuxer requires a session and to be a admin of the server
	// being managed, otherwise it will redirect to the homepage
	InitWeb()
	Name() string
}

var Plugins []Plugin

// Register a plugin, should only be called before webserver is started!!!
func RegisterPlugin(plugin Plugin) {
	if Plugins == nil {
		Plugins = []Plugin{plugin}
	} else {
		Plugins = append(Plugins, plugin)
	}
	common.AddPlugin(plugin)
}
