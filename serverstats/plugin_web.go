package serverstats

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strings"
)

type FormData struct {
	Public         bool
	IgnoreChannels []string `valid:"channel,false"`
}

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/assets/serverstats.html")))

	cpGetHandler := web.RequireGuildChannelsMiddleware(web.ControllerHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats"))
	web.CPMux.Handle(pat.Get("/stats"), cpGetHandler)

	web.CPMux.Handle(pat.Post("/stats/settings"), web.RequireGuildChannelsMiddleware(web.ControllerPostHandler(HandleStatsSettings, cpGetHandler, FormData{}, "Updated serverstats settings")))
	web.CPMux.Handle(pat.Get("/stats/full"), web.APIHandler(publicHandlerJson(HandleStatsJson, false)))

	// Public
	web.ServerPublicMux.Handle(pat.Get("/stats"), web.RequireGuildChannelsMiddleware(web.ControllerHandler(publicHandler(HandleStatsHtml, true), "cp_serverstats")))
	web.ServerPublicMux.Handle(pat.Get("/stats/full"), web.APIHandler(publicHandlerJson(HandleStatsJson, true)))
}

type publicHandlerFunc func(w http.ResponseWriter, r *http.Request, publicAccess bool) (web.TemplateData, error)

func publicHandler(inner publicHandlerFunc, public bool) web.ControllerHandlerFunc {
	mw := func(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
		return inner(w, r.WithContext(web.SetContextTemplateData(r.Context(), map[string]interface{}{"Public": public})), public)
	}

	return mw
}

// Somewhat dirty - should clean up this mess sometime
func HandleStatsHtml(w http.ResponseWriter, r *http.Request, isPublicAccess bool) (web.TemplateData, error) {
	_, activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	var config ServerStatsConfig
	err := configstore.Cached.GetGuildConfig(r.Context(), activeGuild.ID, &config)
	if err != nil && err != configstore.ErrNotFound {
		return templateData, common.ErrWithCaller(err)
	}

	// publicEnabled, _ := client.Cmd("GET", "stats_settings_public:"+activeGuild.ID).Bool()

	templateData["Config"] = config

	return templateData, nil
}

func HandleStatsSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, ag, templateData := web.GetBaseCPContextData(r.Context())

	formData := r.Context().Value(common.ContextKeyParsedForm).(*FormData)

	newConf := &ServerStatsConfig{
		GuildConfigModel: configstore.GuildConfigModel{
			GuildID: common.MustParseInt(ag.ID),
		},
		Public:         formData.Public,
		IgnoreChannels: strings.Join(formData.IgnoreChannels, ","),
	}

	err := configstore.SQL.SetGuildConfig(r.Context(), newConf)
	return templateData, err
}

type publicHandlerFuncJson func(w http.ResponseWriter, r *http.Request, publicAccess bool) interface{}

func publicHandlerJson(inner publicHandlerFuncJson, public bool) web.CustomHandlerFunc {
	mw := func(w http.ResponseWriter, r *http.Request) interface{} {
		return inner(w, r.WithContext(web.SetContextTemplateData(r.Context(), map[string]interface{}{"Public": public})), public)
	}

	return mw
}

func HandleStatsJson(w http.ResponseWriter, r *http.Request, isPublicAccess bool) interface{} {
	client, activeGuild, _ := web.GetBaseCPContextData(r.Context())

	conf, err := GetConfig(r.Context(), activeGuild.ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving stats config")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	if !conf.Public && isPublicAccess {
		return nil
	}

	stats, err := RetrieveFullStats(client, activeGuild.ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving stats")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	return stats
}
