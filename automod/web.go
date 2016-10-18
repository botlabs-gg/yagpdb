package automod

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
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
	web.CPMux.HandleC(pat.New("/automod/*"), autmodMux)
	web.CPMux.HandleC(pat.New("/automod"), autmodMux)

	// Alll handlers here require guild channels present
	autmodMux.UseC(web.RequireFullGuildMW)
	autmodMux.UseC(web.RequireGuildChannelsMiddleware)

	getHandler := web.RenderHandler(HandleAutomod, "cp_automod")

	autmodMux.HandleC(pat.Get("/"), getHandler)
	autmodMux.HandleC(pat.Get(""), getHandler)

	// Post handlers
	autmodMux.HandleC(pat.Post("/general"), web.FormParserMW(web.RenderHandler(HandlePostGeneral, "cp_automod"), GeneralForm{}))

	autmodMux.HandleC(pat.Post("/spam"), web.SimpleConfigSaverHandler(SpamRule{}, getHandler))
	autmodMux.HandleC(pat.Post("/mention"), web.SimpleConfigSaverHandler(MentionRule{}, getHandler))
	autmodMux.HandleC(pat.Post("/invite"), web.SimpleConfigSaverHandler(InviteRule{}, getHandler))
	autmodMux.HandleC(pat.Post("/words"), web.SimpleConfigSaverHandler(WordsRule{}, getHandler))
	autmodMux.HandleC(pat.Post("/sites"), web.SimpleConfigSaverHandler(SitesRule{}, getHandler))
}

func HandleAutomod(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, g, templateData := web.GetBaseCPContextData(ctx)

	spam, mention, invite, words, sites, err := GetRules(g.ID, client)
	web.CheckErr(templateData, err, "Failed retrieving rules", log.Error)

	enabled, err := GetEnabled(g.ID, client)
	web.CheckErr(templateData, err, "Failed checking enabled", log.Error)

	templateData["Enabled"] = enabled
	templateData["Spam"] = spam
	templateData["Mention"] = mention
	templateData["Invite"] = invite
	templateData["Words"] = words
	templateData["Sites"] = sites

	templateData["VisibleURL"] = "/cp/" + g.ID + "/automod/"

	return templateData
}

func HandlePostGeneral(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	form := ctx.Value(web.ContextKeyParsedForm).(*GeneralForm)
	ok := ctx.Value(web.ContextKeyFormOk).(bool)
	if !ok {
		return HandleAutomod(ctx, w, r)
	}
	err := client.Cmd("SET", KeyEnabled(activeGuild.ID), form.Enabled).Err
	if !web.CheckErr(templateData, err, "Failed saving general settings", log.Error) {
		templateData.AddAlerts(web.SucessAlert("Sucessfully saved general settings"))
	}

	return HandleAutomod(ctx, w, r)
}
