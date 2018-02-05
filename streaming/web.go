package streaming

import (
	"context"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
)

type ConextKey int

const (
	ConextKeyConfig ConextKey = iota
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/streaming.html"))

	streamingMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/streaming/*"), streamingMux)
	web.CPMux.Handle(pat.New("/streaming"), streamingMux)

	// Alll handlers here require guild channels present
	streamingMux.Use(web.RequireGuildChannelsMiddleware)
	streamingMux.Use(web.RequireFullGuildMW)
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
		client, guild, tmpl := web.GetBaseCPContextData(r.Context())
		config, err := GetConfig(client, guild.ID)
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
	client, guild, tmpl := web.GetBaseCPContextData(ctx)
	tmpl["VisibleURL"] = "/manage/" + guild.ID + "/streaming/"

	ok := ctx.Value(common.ContextKeyFormOk).(bool)
	newConf := ctx.Value(common.ContextKeyParsedForm).(*Config)

	tmpl["StreamingConfig"] = newConf

	if !ok {
		return tmpl
	}

	err := newConf.Save(client, guild.ID)
	if web.CheckErr(tmpl, err, "Failed saving config :'(", web.CtxLogger(ctx).Error) {
		return tmpl
	}

	err = pubsub.Publish(client, "update_streaming", guild.ID, nil)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("Failed sending update streaming event")
	}

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, guild.ID, "Updated streaming config.")

	return tmpl.AddAlerts(web.SucessAlert("Saved settings"))
}
