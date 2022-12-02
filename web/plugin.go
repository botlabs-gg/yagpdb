package web

import (
	"html/template"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/common"
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

type RenderedServerHomeWidget struct {
	Body    template.HTML
	Title   template.HTML
	Enabled bool
}

type PluginWithServerHomeWidget interface {
	LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (TemplateData, error)
}

type PluginWithServerHomeWidgetMiddlewares interface {
	PluginWithServerHomeWidget
	ServerHomeWidgetApplyMiddlewares(inner http.Handler) http.Handler
}

type ServerHomeWidgetWithOrder interface {
	ServerHomeWidgetOrder() int
}
