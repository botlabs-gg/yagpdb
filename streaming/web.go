package streaming

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
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
	web.CPMux.HandleC(pat.New("/streaming/*"), streamingMux)
	web.CPMux.HandleC(pat.New("/streaming"), streamingMux)

	// Alll handlers here require guild channels present
	streamingMux.UseC(web.RequireGuildChannelsMiddleware)
	streamingMux.UseC(web.RequireFullGuildMW)
	streamingMux.UseC(baseData)

	// Get just renders the template, so let the renderhandler do all the work
	streamingMux.HandleC(pat.Get(""), web.RenderHandler(nil, "cp_streaming"))
	streamingMux.HandleC(pat.Get("/"), web.RenderHandler(nil, "cp_streaming"))

	streamingMux.HandleC(pat.Post(""), web.FormParserMW(web.RenderHandler(HandlePostStreaming, "cp_streaming"), Config{}))
	streamingMux.HandleC(pat.Post("/"), web.FormParserMW(web.RenderHandler(HandlePostStreaming, "cp_streaming"), Config{}))
}

// Adds the current config to the context
func baseData(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		client, guild, tmpl := web.GetBaseCPContextData(ctx)
		config, err := GetConfig(client, guild.ID)
		if web.CheckErr(tmpl, err, "Failed retrieving streaming config :'(", logrus.Error) {
			web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_streaming", tmpl))
			return
		}
		tmpl["StreamingConfig"] = config
		inner.ServeHTTPC(context.WithValue(ctx, ConextKeyConfig, config), w, r)
	}

	return goji.HandlerFunc(mw)
}

func HandlePostStreaming(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, guild, tmpl := web.GetBaseCPContextData(ctx)

	ok := ctx.Value(web.ContextKeyFormOk).(bool)
	newConf := ctx.Value(web.ContextKeyParsedForm).(*Config)

	tmpl["StreamingConfig"] = newConf

	if !ok {
		return tmpl
	}

	err := newConf.Save(client, guild.ID)
	if web.CheckErr(tmpl, err, "Failed saving config :'(", logrus.Error) {
		return tmpl
	}

	err = bot.PublishEvent(client, "update_streaming", guild.ID, nil)
	if err != nil {
		logrus.WithError(err).Error("Failed sending update streaming event")
	}
	return tmpl.AddAlerts(web.SucessAlert("Saved settings"))
}
