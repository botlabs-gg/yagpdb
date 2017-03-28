package moderation

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/assets/moderation.html")))

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/moderation"), subMux)
	web.CPMux.Handle(pat.New("/moderation/*"), subMux)

	subMux.Use(web.RequireGuildChannelsMiddleware)
	subMux.Use(web.RequireFullGuildMW)

	subMux.Use(web.RequireBotMemberMW) // need the bot's role
	subMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageMessages, discordgo.PermissionEmbedLinks))

	getHandler := web.ControllerHandler(HandleModeration, "cp_moderation")
	postHandler := web.ControllerPostHandler(HandlePostModeration, getHandler, Config{}, "Updated moderation config")

	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)
	subMux.Handle(pat.Post(""), postHandler)
	subMux.Handle(pat.Post("/"), postHandler)
}

// The moderation page itself
func HandleModeration(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	if _, ok := templateData["ModConfig"]; !ok {
		config, err := GetConfig(activeGuild.ID)
		if err != nil {
			return templateData, err
		}
		templateData["ModConfig"] = config
	}

	return templateData, nil
}

// Update the settings
func HandlePostModeration(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/moderation/"

	newConfig := ctx.Value(common.ContextKeyParsedForm).(*Config)
	templateData["ModConfig"] = newConfig

	err := newConfig.Save(client, activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	return templateData, nil
}
