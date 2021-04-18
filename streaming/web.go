package streaming

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/cplogs"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
)

type ConextKey int

const (
	ConextKeyConfig ConextKey = iota
)

var panelLogKey = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "streaming_settings_updated", FormatString: "Updated streaming settings"})

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../streaming/assets/streaming.html", "templates/plugins/streaming.html")
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "Streaming",
		URL:  "streaming",
		Icon: "fas fa-video",
	})

	streamingMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/streaming/*"), streamingMux)
	web.CPMux.Handle(pat.New("/streaming"), streamingMux)

	// Alll handlers here require guild channels present
	streamingMux.Use(web.RequireBotMemberMW)
	streamingMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles))
	streamingMux.Use(baseData)

	// Get just renders the template, so let the renderhandler do all the work
	streamingMux.Handle(pat.Get(""), web.RenderHandler(nil, "cp_streaming"))
	streamingMux.Handle(pat.Get("/"), web.RenderHandler(nil, "cp_streaming"))

	streamingMux.Handle(pat.Post(""), web.FormParserMW(web.RenderHandler(HandlePostStreaming, "cp_streaming"), Config{}))
	streamingMux.Handle(pat.Post("/"), web.FormParserMW(web.RenderHandler(HandlePostStreaming, "cp_streaming"), Config{}))
}

// Adds the current config to the context
func baseData(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		guild, tmpl := web.GetBaseCPContextData(r.Context())
		config, err := GetConfig(guild.ID)
		if web.CheckErr(tmpl, err, "Failed retrieving streaming config :'(", web.CtxLogger(r.Context()).Error) {
			web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_streaming", tmpl))
			return
		}
		tmpl["StreamingConfig"] = config
		inner.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ConextKeyConfig, config)))
	}

	return http.HandlerFunc(mw)
}

func HandlePostStreaming(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	guild, tmpl := web.GetBaseCPContextData(ctx)
	tmpl["VisibleURL"] = "/manage/" + discordgo.StrID(guild.ID) + "/streaming/"

	ok := ctx.Value(common.ContextKeyFormOk).(bool)
	newConf := ctx.Value(common.ContextKeyParsedForm).(*Config)

	tmpl["StreamingConfig"] = newConf

	if !ok {
		return tmpl
	}

	err := newConf.Save(guild.ID)
	if web.CheckErr(tmpl, err, "Failed saving config :'(", web.CtxLogger(ctx).Error) {
		return tmpl
	}

	err = featureflags.UpdatePluginFeatureFlags(guild.ID, &Plugin{})
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("failed updating feature flags")
	}

	err = pubsub.Publish("update_streaming", guild.ID, nil)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("Failed sending update streaming event")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKey))

	return tmpl.AddAlerts(web.SucessAlert("Saved settings"))
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Streaming"
	templateData["SettingsPath"] = "/streaming"

	config, err := GetConfig(ag.ID)
	if err != nil {
		return templateData, err
	}

	format := `<ul>
	<li>Streaming status: %s</li>
	<li>Streaming role: <code>%s</code>%s</li>
	<li>Streaming message: <code>#%s</code>%s</li>
</ul>`

	status := web.EnabledDisabledSpanStatus(config.Enabled)

	if config.Enabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	roleStr := "none / unknown"
	indicatorRole := ""
	if role := ag.Role(config.GiveRole); role != nil {
		roleStr = html.EscapeString(role.Name)
		indicatorRole = web.Indicator(true)
	} else {
		indicatorRole = web.Indicator(false)
	}

	indicatorMessage := ""
	channelStr := "none / unknown"

	if channel := ag.Channel(config.AnnounceChannel); channel != nil {
		indicatorMessage = web.Indicator(true)
		channelStr = html.EscapeString(channel.Name)
	} else {
		indicatorMessage = web.Indicator(false)
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, status, roleStr, indicatorRole, channelStr, indicatorMessage))

	return templateData, nil
}
