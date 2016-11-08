package serverstats

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/serverstats.html"))
	web.CPMux.HandleC(pat.Get("/stats"), web.RenderHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats"))
	web.CPMux.HandleC(pat.Get("/stats/"), web.RenderHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats"))

	web.CPMux.HandleC(pat.Post("/stats/settings"), web.RenderHandler(HandleStatsSettings, "cp_serverstats"))
	web.CPMux.HandleC(pat.Get("/stats/full"), web.APIHandler(publicHandler(HandleStatsJson, false)))

	// Public
	web.ServerPublicMux.HandleC(pat.Get("/stats"), web.RenderHandler(publicHandler(HandleStatsHtml, true), "cp_serverstats"))
	web.ServerPublicMux.HandleC(pat.Get("/stats/full"), web.APIHandler(publicHandler(HandleStatsJson, true)))
}

type publicHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, publicAccess bool) interface{}

func publicHandler(inner publicHandlerFunc, public bool) web.CustomHandlerFunc {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
		return inner(web.SetContextTemplateData(ctx, map[string]interface{}{"Public": public}), w, r, public)
	}

	return mw
}

// Somewhat dirty - should clean up this mess sometime
func HandleStatsHtml(ctx context.Context, w http.ResponseWriter, r *http.Request, isPublicAccess bool) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	publicEnabled, _ := client.Cmd("GET", "stats_settings_public:"+activeGuild.ID).Bool()
	templateData["PublicEnabled"] = publicEnabled

	return templateData
}

func HandleStatsSettings(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	public := r.FormValue("public") == "on"

	current, _ := client.Cmd("GET", "stats_settings_public:"+activeGuild.ID).Bool()
	err := client.Cmd("SET", "stats_settings_public:"+activeGuild.ID, public).Err

	if err != nil {
		log.WithError(err).Error("Failed saving stats settings to redis")
		templateData.AddAlerts(web.ErrorAlert("Failed saving setting..."))
		templateData["PublicEnabled"] = current
	} else {
		templateData["PublicEnabled"] = public
	}

	templateData["GuildName"] = activeGuild.Name
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/stats/"

	return templateData
}

func HandleStatsJson(ctx context.Context, w http.ResponseWriter, r *http.Request, isPublicAccess bool) interface{} {
	client, activeGuild, _ := web.GetBaseCPContextData(ctx)

	stats, err := RetrieveFullStats(client, activeGuild.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving stats")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	return stats
}
