package serverstats

import (
	"encoding/json"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"log"
	"net/http"
)

func (p *Plugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/serverstats.html"))
	cpMux.HandleFuncC(pat.Get("/cp/:server/stats"), HandleStatsHtml)
	cpMux.HandleFuncC(pat.Get("/cp/:server/stats/"), HandleStatsHtml)
	cpMux.HandleFuncC(pat.Get("/cp/:server/stats/full"), HandleStatsJson)
}

func HandleStatsHtml(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "serverstats"

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_serverstats", templateData))
}

func HandleStatsJson(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, _ := web.GetBaseCPContextData(ctx)
	stats, err := RetrieveFullStats(client, activeGuild.ID)
	if err != nil {
		log.Println("Failed retrieving stats", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	out, err := json.Marshal(stats)
	if err != nil {
		log.Println("Failed Encoding stats", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(out)
}
