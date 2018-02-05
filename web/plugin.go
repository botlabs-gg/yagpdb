package web

import (
	"github.com/jonas747/yagpdb/common"
)

// Plugin represents a web plugin
type Plugin interface {
	common.Plugin

	// Parse the templates and set up the http routes here
	// mainMuxer is the root and cpmuxer handles the /cp/ route
	// the cpmuxer requires a session and to be a admin of the server
	// being managed, otherwise it will redirect to the homepage
	InitWeb()
}
