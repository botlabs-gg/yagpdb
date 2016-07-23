package serverstats

import (
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

type WebPlugin struct{}

func RegisterPlugin() {
	web.RegisterPlugin(&WebPlugin{})
}

func (p *WebPlugin) Name() string {
	return "Server stats"
}

func (p *WebPlugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/serverstats.html"))
	cpMux.HandleFuncC(pat.Get("/cp/:server/stats"), HandleStats)
	cpMux.HandleFuncC(pat.Get("/cp/:server/stats/"), HandleStats)
}

func HandleStats(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "serverstats"

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_serverstats", templateData))
}
