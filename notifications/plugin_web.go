package notifications

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/notifications_general.html"))

	getHandler := web.RenderHandler(HandleNotificationsGet, "cp_notifications_general")
	postHandler := web.ControllerPostHandler(HandleNotificationsPost, getHandler, Config{}, "Updated general notifiactions config.")

	web.CPMux.HandleC(pat.Get("/notifications/general"), web.RequireGuildChannelsMiddleware(getHandler))
	web.CPMux.HandleC(pat.Get("/notifications/general/"), web.RequireGuildChannelsMiddleware(getHandler))

	web.CPMux.HandleC(pat.Post("/notifications/general"), web.RequireGuildChannelsMiddleware(postHandler))
	web.CPMux.HandleC(pat.Post("/notifications/general/"), web.RequireGuildChannelsMiddleware(postHandler))
}

func HandleNotificationsGet(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	formConfig, ok := ctx.Value(web.ContextKeyParsedForm).(*Config)
	if ok {
		templateData["NotifyConfig"] = formConfig
	} else {
		templateData["NotifyConfig"] = GetConfig(client, activeGuild.ID)
	}

	return templateData
}

func HandleNotificationsPost(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/notifications/general/"

	newConfig := ctx.Value(web.ContextKeyParsedForm).(*Config)

	err := common.SetRedisJson(client, "notifications/general:"+activeGuild.ID, newConfig)
	if err != nil {
		return nil, err
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully saved everything! :')"))

	return templateData, nil
}
