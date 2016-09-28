package streaming

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
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

	streamingMux.HandleC(pat.Post(""), web.RenderHandler(HandlePostStreaming, "cp_streaming"))
	streamingMux.HandleC(pat.Post("/"), web.RenderHandler(HandlePostStreaming, "cp_streaming"))
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

	newConf := &Config{
		Enabled: r.FormValue("enabled") == "on",

		AnnounceChannel: r.FormValue("announce_channel"),
		AnnounceMessage: r.FormValue("announce_message"),

		GiveRole:    r.FormValue("give_role"),
		RequireRole: r.FormValue("require_role"),
		IgnoreRole:  r.FormValue("ignore_role"),
	}

	oldConf := ctx.Value(ConextKeyConfig).(*Config)
	sendConf := *newConf
	tmpl["StreamingConfig"] = sendConf

	// Validate the message
	if newConf.AnnounceMessage != "" {
		_, err := common.ParseExecuteTemplate(newConf.AnnounceMessage, nil)
		if web.CheckErr(tmpl, err, "", nil) {
			newConf.AnnounceMessage = oldConf.AnnounceMessage
			return tmpl
		}
	}

	err := newConf.Save(client, guild.ID)
	web.CheckErr(tmpl, err, "Failed saving config :'(", logrus.Error)
	return tmpl
}
