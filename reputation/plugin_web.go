package reputation

import (
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"strconv"
)

func (p *Plugin) InitWeb(root *goji.Mux, cp *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/reputation.html"))

	cp.HandleFuncC(pat.Get("/cp/:server/reputation"), HandleGetReputation)
	cp.HandleFuncC(pat.Get("/cp/:server/reputation/"), HandleGetReputation)
	cp.HandleFuncC(pat.Post("/cp/:server/reputation"), HandlePostReputation)
	cp.HandleFuncC(pat.Post("/cp/:server/reputation/"), HandlePostReputation)
}

func HandleGetReputation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	defer func() {
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_reputation", templateData))
	}()

	settings, err := GetFullSettings(client, activeGuild.ID)
	if web.CheckErr(templateData, err) {
		return
	}

	templateData["Settings"] = settings
}

func HandlePostReputation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	defer func() {
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_reputation", templateData))
	}()

	currentSettings, err := GetFullSettings(client, activeGuild.ID)
	if web.CheckErr(templateData, err) {
		return
	}

	templateData["Settings"] = currentSettings

	parsed, err := strconv.ParseInt(r.FormValue("cooldown"), 10, 32)
	if web.CheckErr(templateData, err) {
		return
	}

	newSettings := &Settings{
		Enabled:  r.FormValue("enabled") == "on",
		Cooldown: int(parsed),
	}

	err = newSettings.Save(client, activeGuild.ID)
	if web.CheckErr(templateData, err) {
		return
	}

	templateData["Settings"] = newSettings
}
