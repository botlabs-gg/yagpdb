package reputation

import (
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"strconv"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/reputation.html"))

	web.CPMux.HandleC(pat.Get("/cp/:server/reputation"), web.RenderHandler(HandleGetReputation, "cp_reputation"))
	web.CPMux.HandleC(pat.Get("/cp/:server/reputation/"), web.RenderHandler(HandleGetReputation, "cp_reputation"))
	web.CPMux.HandleC(pat.Post("/cp/:server/reputation"), web.RenderHandler(HandlePostReputation, "cp_reputation"))
	web.CPMux.HandleC(pat.Post("/cp/:server/reputation/"), web.RenderHandler(HandlePostReputation, "cp_reputation"))
}

func HandleGetReputation(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	settings, err := GetFullSettings(client, activeGuild.ID)
	if !web.CheckErr(templateData, err) {
		templateData["settings"] = settings
	}
	return templateData
}

func HandlePostReputation(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentSettings, err := GetFullSettings(client, activeGuild.ID)
	if web.CheckErr(templateData, err) {
		return templateData
	}

	templateData["Settings"] = currentSettings

	parsed, err := strconv.ParseInt(r.FormValue("cooldown"), 10, 32)
	if web.CheckErr(templateData, err) {
		return templateData
	}

	newSettings := &Settings{
		Enabled:  r.FormValue("enabled") == "on",
		Cooldown: int(parsed),
	}

	err = newSettings.Save(client, activeGuild.ID)
	if web.CheckErr(templateData, err) {
		return templateData
	}

	templateData["Settings"] = newSettings
	return templateData
}
