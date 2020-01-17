package moderation

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
)

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../moderation/assets/moderation.html", "templates/plugins/moderation.html")

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Moderation",
		URL:  "moderation",
		Icon: "fas fa-gavel",
	})

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/moderation"), subMux)
	web.CPMux.Handle(pat.New("/moderation/*"), subMux)

	subMux.Use(web.RequireGuildChannelsMiddleware)

	subMux.Use(web.RequireBotMemberMW) // need the bot's role
	subMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageMessages, discordgo.PermissionEmbedLinks))

	getHandler := web.ControllerHandler(HandleModeration, "cp_moderation")
	postHandler := web.ControllerPostHandler(HandlePostModeration, getHandler, Config{}, "Updated moderation config")
	clearServerWarnings := web.ControllerPostHandler(HandleClearServerWarnings, getHandler, nil, "Cleared all server warnings")

	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)
	subMux.Handle(pat.Post(""), postHandler)
	subMux.Handle(pat.Post("/"), postHandler)
	subMux.Handle(pat.Post("/clear_server_warnings"), clearServerWarnings)
}

// The moderation page itself
func HandleModeration(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["DefaultDMMessage"] = DefaultDMMessage

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
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/moderation/"

	newConfig := ctx.Value(common.ContextKeyParsedForm).(*Config)
	templateData["ModConfig"] = newConfig

	err := newConfig.Save(activeGuild.ID)

	templateData["DefaultDMMessage"] = DefaultDMMessage

	return templateData, err
}

// Clear all server warnigns
func HandleClearServerWarnings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/moderation/"

	rows := common.GORM.Where("guild_id = ?", activeGuild.ID).Delete(WarningModel{}).RowsAffected
	templateData.AddAlerts(web.SucessAlert("Deleted ", rows, " warnings!"))
	templateData["DefaultDMMessage"] = DefaultDMMessage

	return templateData, nil
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Moderation"
	templateData["SettingsPath"] = "/moderation"

	config, err := GetConfig(activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	format := `<ul>
	<li>Report command: %s</li>
	<li>Clean command: %s</li>
	<li>Giverole/Takerole commands: %s</li>
	<li>Kick command: %s</li>
	<li>Ban command: %s</li>
	<li>Mute/Unmute commands: %s</li>
	<li>Warning commands: %s</li>
</ul>`

	if config.ReportEnabled || config.CleanEnabled || config.GiveRoleCmdEnabled || config.ActionChannel != "" ||
		config.MuteEnabled || config.KickEnabled || config.BanEnabled || config.WarnCommandsEnabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, web.EnabledDisabledSpanStatus(config.ReportEnabled),
		web.EnabledDisabledSpanStatus(config.CleanEnabled), web.EnabledDisabledSpanStatus(config.GiveRoleCmdEnabled),
		web.EnabledDisabledSpanStatus(config.KickEnabled), web.EnabledDisabledSpanStatus(config.BanEnabled),
		web.EnabledDisabledSpanStatus(config.MuteEnabled), web.EnabledDisabledSpanStatus(config.WarnCommandsEnabled)))

	return templateData, nil
}
