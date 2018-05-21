package notifications

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	tmplPath := "templates/plugins/notifications_general.html"
	if common.Testing {
		tmplPath = "../../notifications/assets/notifications_general.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPath))

	getHandler := web.RenderHandler(HandleNotificationsGet, "cp_notifications_general")
	postHandler := web.ControllerPostHandler(HandleNotificationsPost, getHandler, Config{}, "Updated general notifications config.")

	web.CPMux.Handle(pat.Get("/notifications/general"), web.RequireGuildChannelsMiddleware(getHandler))
	web.CPMux.Handle(pat.Get("/notifications/general/"), web.RequireGuildChannelsMiddleware(getHandler))

	web.CPMux.Handle(pat.Post("/notifications/general"), web.RequireGuildChannelsMiddleware(postHandler))
	web.CPMux.Handle(pat.Post("/notifications/general/"), web.RequireGuildChannelsMiddleware(postHandler))
}

func HandleNotificationsGet(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	_, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	formConfig, ok := ctx.Value(common.ContextKeyParsedForm).(*Config)
	if ok {
		templateData["NotifyConfig"] = formConfig
	} else {
		templateData["NotifyConfig"] = GetConfig(activeGuild.ID)
	}

	return templateData
}

func HandleNotificationsPost(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/notifications/general/"

	newConfig := ctx.Value(common.ContextKeyParsedForm).(*Config)

	newConfig.GuildID = activeGuild.ID

	err := configstore.SQL.SetGuildConfig(ctx, newConfig)
	if err != nil {
		return templateData, nil
	}

	return templateData, nil
}
