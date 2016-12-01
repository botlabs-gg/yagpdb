package moderation

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/moderation.html"))

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/moderation"), subMux)
	web.CPMux.Handle(pat.New("/moderation/*"), subMux)

	subMux.UseC(web.RequireGuildChannelsMiddleware)
	subMux.UseC(web.RequireFullGuildMW)

	subMux.UseC(web.RequireBotMemberMW) // need the bot's role
	subMux.UseC(web.RequirePermMW(discordgo.PermissionManageRoles, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageMessages, discordgo.PermissionEmbedLinks))

	getHandler := web.ControllerHandler(HandleModeration, "cp_moderation")
	postHandler := web.ControllerPostHandler(HandlePostModeration, getHandler, Config{}, "Updated moderation config")

	subMux.HandleC(pat.Get(""), getHandler)
	subMux.HandleC(pat.Get("/"), getHandler)
	subMux.HandleC(pat.Post(""), postHandler)
	subMux.HandleC(pat.Post("/"), postHandler)
}

// The moderation page itself
func HandleModeration(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, activeGuild, templateData := web.GetBaseCPContextData(ctx)

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
func HandlePostModeration(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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
