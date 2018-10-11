package serverstats

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

type FormData struct {
	Public         bool
	IgnoreChannels []int64 `valid:"channel,false"`
}

func (p *Plugin) InitWeb() {
	tmplPath := "templates/plugins/serverstats.html"
	if common.Testing {
		tmplPath = "../../serverstats/assets/serverstats.html"
	}
	web.Templates = template.Must(web.Templates.ParseFiles(tmplPath))

	statsCPMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/stats"), statsCPMux)
	web.CPMux.Handle(pat.New("/stats/*"), statsCPMux)
	statsCPMux.Use(web.RequireGuildChannelsMiddleware)

	cpGetHandler := web.ControllerHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats")
	statsCPMux.Handle(pat.Get(""), cpGetHandler)
	statsCPMux.Handle(pat.Get("/"), cpGetHandler)

	statsCPMux.Handle(pat.Post("/settings"), web.ControllerPostHandler(HandleSaveStatsSettings, cpGetHandler, FormData{}, "Updated serverstats settings"))
	statsCPMux.Handle(pat.Get("/full"), web.APIHandler(publicHandlerJson(HandleStatsJson, false)))

	// Public
	web.ServerPublicMux.Handle(pat.Get("/stats"), web.RequireGuildChannelsMiddleware(web.ControllerHandler(publicHandler(HandleStatsHtml, true), "cp_serverstats")))
	web.ServerPublicMux.Handle(pat.Get("/stats/full"), web.RequireGuildChannelsMiddleware(web.APIHandler(publicHandlerJson(HandleStatsJson, true))))
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
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	config, err := GetConfig(r.Context(), activeGuild.ID)
	if err != nil {
		return templateData, common.ErrWithCaller(err)
	}

	templateData["Config"] = config
	templateData["ExtraHead"] = template.HTML(`
<link rel="stylesheet" href="/static/vendor/morris/morris.css" />
<link rel="stylesheet" href="/static/vendor/chartist/chartist.min.css" />
	`)

	return templateData, nil
}

func HandleSaveStatsSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	formData := r.Context().Value(common.ContextKeyParsedForm).(*FormData)

	stringedChannels := ""
	alreadyAdded := make([]int64, 0, len(formData.IgnoreChannels))
OUTER:
	for i, v := range formData.IgnoreChannels {
		// only add each once
		for _, ad := range alreadyAdded {
			if ad == v {
				continue OUTER
			}
		}

		// make sure the channel exists
		channelExists := false
		for _, ec := range ag.Channels {
			if ec.ID == v {
				channelExists = true
				break
			}
		}

		if !channelExists {
			continue
		}

		if i != 0 {
			stringedChannels += ","
		}

		alreadyAdded = append(alreadyAdded, v)
		stringedChannels += strconv.FormatInt(v, 10)
	}

	model := &models.ServerStatsConfig{
		GuildID:        ag.ID,
		Public:         null.BoolFrom(formData.Public),
		IgnoreChannels: null.StringFrom(stringedChannels),
		CreatedAt:      null.TimeFrom(time.Now()),
	}

	err := model.UpsertG(r.Context(), true, []string{"guild_id"}, boil.Whitelist("public", "ignore_channels"), boil.Infer())
	if err == nil {
		go pubsub.Publish("server_stats_invalidate_cache", ag.ID, nil)
	}

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
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	conf, err := GetConfig(r.Context(), activeGuild.ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving stats config")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	if !conf.Public && isPublicAccess {
		return nil
	}

	stats, err := RetrieveFullStats(activeGuild.ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving stats")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	// Update the names to human readable ones, leave the ids in the name fields for the ones not available
	for _, cs := range stats.ChannelsHour {
		for _, channel := range activeGuild.Channels {
			if discordgo.StrID(channel.ID) == cs.Name {
				cs.Name = channel.Name
				break
			}
		}
	}

	return stats
}
