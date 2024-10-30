package serverstats

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/serverstats/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/karlseguin/rcache"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/serverstats.html
var PageHTML string

var WebStatsCache = rcache.New(cacheChartFetcher, time.Minute)
var WebConfigCache = rcache.NewInt(cacheConfigFetcher, time.Minute)

var panelLogKey = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "serverstats_settings_updated", FormatString: "Updated serverstats settings"})

type FormData struct {
	Public         bool
	IgnoreChannels []int64 `valid:"channel,false"`
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("serverstats/assets/serverstats.html", PageHTML)

	web.AddSidebarItem(web.SidebarCategoryTopLevel, &web.SidebarItem{
		Name: "Stats",
		URL:  "stats",
		Icon: "fas fa-chart-bar",
	})

	statsCPMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/stats"), statsCPMux)
	web.CPMux.Handle(pat.New("/stats/*"), statsCPMux)

	cpGetHandler := web.ControllerHandler(publicHandler(HandleStatsHtml, false), "cp_serverstats")
	statsCPMux.Handle(pat.Get(""), cpGetHandler)
	statsCPMux.Handle(pat.Get("/"), cpGetHandler)

	statsCPMux.Handle(pat.Post("/settings"), web.ControllerPostHandler(HandleSaveStatsSettings, cpGetHandler, FormData{}))
	statsCPMux.Handle(pat.Get("/daily_json"), web.APIHandler(publicHandlerJson(HandleStatsJson, false)))
	statsCPMux.Handle(pat.Get("/charts"), web.APIHandler(publicHandlerJson(HandleStatsCharts, false)))

	// Public
	web.ServerPublicMux.Handle(pat.Get("/stats"), web.ControllerHandler(publicHandler(HandleStatsHtml, true), "cp_serverstats"))
	web.ServerPublicMux.Handle(pat.Get("/stats/daily_json"), web.APIHandler(publicHandlerJson(HandleStatsJson, true)))
	web.ServerPublicMux.Handle(pat.Get("/stats/charts"), web.APIHandler(publicHandlerJson(HandleStatsCharts, true)))
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

	if confDeprecated.GetBool() {
		templateData.AddAlerts(web.WarningAlert("Serverstats are deprecated in favor of the superior discord server insights. Recording of new stats may stop at any time and stats will no longer be available next month."))
	}

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
		pubsub.EvictCacheSet(cachedConfig, ag.ID)
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKey))
	}

	WebConfigCache.Delete(int(ag.ID))

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

	conf := GetConfigWeb(activeGuild.ID)
	if conf == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	if !conf.Public && isPublicAccess {
		return nil
	}

	stats, err := RetrieveDailyStats(time.Now(), activeGuild.ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving stats")
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	// Update the names to human readable ones, leave the ids in the name fields for the ones not available
	for _, cs := range stats.ChannelMessages {
		for _, channel := range activeGuild.Channels {
			if discordgo.StrID(channel.ID) == cs.Name {
				cs.Name = channel.Name
				break
			}
		}
	}

	return stats
}

type ChartResponse struct {
	Days int                `json:"days"`
	Data []*ChartDataPeriod `json:"data"`
}

func HandleStatsCharts(w http.ResponseWriter, r *http.Request, isPublicAccess bool) interface{} {
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	conf := GetConfigWeb(activeGuild.ID)
	if conf == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	if !conf.Public && isPublicAccess {
		return nil
	}

	numDays := 7
	if r.URL.Query().Get("days") != "" {
		numDays, _ = strconv.Atoi(r.URL.Query().Get("days"))
		if numDays > 365 {
			numDays = 365
		}
	}

	if !premium.ContextPremium(r.Context()) && (numDays > 7 || numDays <= 0) {
		numDays = 7
	}

	stats := CacheGetCharts(activeGuild.ID, numDays, r.Context())
	return stats
}

func emptyChartData() *ChartResponse {
	return &ChartResponse{
		Days: 0,
		Data: []*ChartDataPeriod{},
	}
}

func CacheGetCharts(guildID int64, days int, ctx context.Context) *ChartResponse {
	if os.Getenv("YAGPDB_SERVERSTATS_DISABLE_SERVERSTATS") != "" {
		return emptyChartData()
	}

	fetchDays := days
	if days < 7 {
		fetchDays = 7
	}

	// default to full time stats
	if days != 30 && days != 365 && days > 7 {
		fetchDays = -1
		days = -1
	} else if days < 1 {
		days = -1
		fetchDays = -1
	}

	key := "charts:" + strconv.FormatInt(guildID, 10) + ":" + strconv.FormatInt(int64(fetchDays), 10)
	statsInterface := WebStatsCache.Get(key)
	if statsInterface == nil {
		return emptyChartData()
	}

	stats := statsInterface.(*ChartResponse)
	cop := *stats
	if fetchDays != days && days != -1 && len(cop.Data) > days {
		cop.Data = cop.Data[:days]
		cop.Days = days
	}

	return &cop
}

func cacheChartFetcher(key string) interface{} {
	split := strings.Split(key, ":")
	if len(split) < 3 {
		logger.Error("invalid cache key: ", key)
		return nil
	}

	guildID, _ := strconv.ParseInt(split[1], 10, 64)
	days, _ := strconv.Atoi(split[2])

	periods, err := RetrieveChartDataPeriods(context.Background(), guildID, time.Now(), days)
	if err != nil {
		logger.WithError(err).WithField("cache_key", key).Error("failed retrieving chart data")
		return nil
	}

	return &ChartResponse{
		Days: days,
		Data: periods,
	}
}

func GetConfigWeb(guildID int64) *ServerStatsConfig {
	config := WebConfigCache.Get(int(guildID))
	if config == nil {
		return nil
	}

	return config.(*ServerStatsConfig)
}

func cacheConfigFetcher(key int) interface{} {
	config, err := GetConfig(context.Background(), int64(key))
	if err != nil {
		logger.WithError(err).WithField("cache_key", key).Error("failed retrieving stats config")
		return nil
	}

	return config
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Server Stats"
	templateData["SettingsPath"] = "/stats"
	templateData["WidgetEnabled"] = true

	config, err := GetConfig(r.Context(), activeGuild.ID)
	if err != nil {
		return templateData, common.ErrWithCaller(err)
	}

	const format = `<ul>
	<li>Public stats: %s</li>
	<li>Disallowed channnels: <code>%d</code></li>
</ul>`

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, web.EnabledDisabledSpanStatus(config.Public), len(config.ParsedChannels)))

	return templateData, nil
}
