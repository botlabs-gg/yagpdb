package serverstats

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/serverstats.html"))
	web.CPMux.Handle(pat.Get("/stats"), web.RenderHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats"))
	web.CPMux.Handle(pat.Get("/stats/"), web.RenderHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats"))

	web.CPMux.Handle(pat.Post("/stats/settings"), web.RenderHandler(HandleStatsSettings, "cp_serverstats"))
	web.CPMux.Handle(pat.Get("/stats/full"), web.APIHandler(publicHandler(HandleStatsJson, false)))

	// Public
	web.ServerPublicMux.Handle(pat.Get("/stats"), web.RenderHandler(publicHandler(HandleStatsHtml, true), "cp_serverstats"))
	web.ServerPublicMux.Handle(pat.Get("/stats/full"), web.APIHandler(publicHandler(HandleStatsJson, true)))
}

type publicHandlerFunc func(w http.ResponseWriter, r *http.Request, publicAccess bool) interface{}

func publicHandler(inner publicHandlerFunc, public bool) web.CustomHandlerFunc {
	mw := func(w http.ResponseWriter, r *http.Request) interface{} {
		return inner(w, r.WithContext(web.SetContextTemplateData(r.Context(), map[string]interface{}{"Public": public})), public)
	}

	return mw
}

// Somewhat dirty - should clean up this mess sometime
func HandleStatsHtml(w http.ResponseWriter, r *http.Request, isPublicAccess bool) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	publicEnabled, _ := client.Cmd("GET", "stats_settings_public:"+activeGuild.ID).Bool()

	templateData["PublicEnabled"] = publicEnabled

	return templateData
}

func HandleStatsSettings(w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(r.Context())

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

func HandleStatsJson(w http.ResponseWriter, r *http.Request, isPublicAccess bool) interface{} {
	client, activeGuild, _ := web.GetBaseCPContextData(r.Context())

	publicEnabled, _ := client.Cmd("GET", "stats_settings_public:"+activeGuild.ID).Bool()
	if !publicEnabled && isPublicAccess {
		return nil
	}

	stats, err := RetrieveFullStats(client, activeGuild.ID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving stats")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	return stats
}
