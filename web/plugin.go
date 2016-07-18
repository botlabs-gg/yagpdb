package web

import (
	"goji.io"
)

// Plugin represents a web plugin
type Plugin interface {
	// Parse the templates and set up the http routes here
	// mainMuxer is the root and cpmuxer handles the /cp/ route
	// the cpmuxer requires a session and to be a admin of the server
	// being managed, otherwise it will redirect to the homepage
	Init(mainMuxer, cpMuxer *goji.Mux)
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
