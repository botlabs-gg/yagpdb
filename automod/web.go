package automod

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
)

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

type GeneralForm struct {
	Enabled bool
}

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/automod.html"))

	autmodMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/automod/*"), autmodMux)
	web.CPMux.Handle(pat.New("/automod"), autmodMux)

	// Alll handlers here require guild channels present
	autmodMux.Use(web.RequireFullGuildMW)
	autmodMux.Use(web.RequireGuildChannelsMiddleware)
	autmodMux.Use(web.RequireBotMemberMW)
	autmodMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageMessages))

	getHandler := web.RenderHandler(HandleAutomod, "cp_automod")

	autmodMux.Handle(pat.Get("/"), getHandler)
	autmodMux.Handle(pat.Get(""), getHandler)

	// Post handlers
	autmodMux.Handle(pat.Post("/"), ExtraPostMW(web.SimpleConfigSaverHandler(Config{}, getHandler)))
	autmodMux.Handle(pat.Post(""), ExtraPostMW(web.SimpleConfigSaverHandler(Config{}, getHandler)))
}

func HandleAutomod(w http.ResponseWriter, r *http.Request) interface{} {
	client, g, templateData := web.GetBaseCPContextData(r.Context())

	config, err := GetConfig(client, g.ID)
	web.CheckErr(templateData, err, "Failed retrieving rules", web.CtxLogger(r.Context()).Error)

	templateData["AutomodConfig"] = config
	templateData["VisibleURL"] = "/cp/" + g.ID + "/automod/"

	return templateData
}

// Invalidates the cache
func ExtraPostMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		client, activeGuild, _ := web.GetBaseCPContextData(r.Context())
		pubsub.Publish(client, "update_automod_rules", activeGuild.ID, nil)
		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}
