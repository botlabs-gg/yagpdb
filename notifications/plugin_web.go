package notifications

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/notifications_general.html"))

	web.CPMux.HandleC(pat.Get("/notifications/general"), web.RequireGuildChannelsMiddleware(web.RenderHandler(HandleNotificationsGet, "cp_notifications_general")))
	web.CPMux.HandleC(pat.Get("/notifications/general/"), web.RequireGuildChannelsMiddleware(web.RenderHandler(HandleNotificationsGet, "cp_notifications_general")))
	web.CPMux.HandleC(pat.Post("/notifications/general"), web.RequireGuildChannelsMiddleware(web.FormParserMW(web.RenderHandler(HandleNotificationsPost, "cp_notifications_general"), Config{})))
	web.CPMux.HandleC(pat.Post("/notifications/general/"), web.RequireGuildChannelsMiddleware(web.FormParserMW(web.RenderHandler(HandleNotificationsPost, "cp_notifications_general"), Config{})))
}

func HandleNotificationsGet(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	templateData["NotifyConfig"] = GetConfig(client, activeGuild.ID)

	return templateData
}

func HandleNotificationsPost(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	newConfig := ctx.Value(web.ContextKeyParsedForm).(*Config)
	ok := ctx.Value(web.ContextKeyFormOk).(bool)

	templateData["NotifyConfig"] = newConfig
	if !ok {
		return templateData
	}

	err := common.SetRedisJson(client, "notifications/general:"+activeGuild.ID, newConfig)
	if web.CheckErr(templateData, err, "Failed saving :(", log.Error) {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully saved everything! :')"))

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Updated general notification settings")

	return templateData
}
