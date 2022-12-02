package moderation

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/moderation.html
var PageHTML string

var (
	panelLogKeyUpdatedSettings = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "moderation_settings_updated", FormatString: "Updated moderation config"})
	panelLogKeyClearWarnings   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "moderation_warnings_cleared", FormatString: "Cleared %d moderation user warnings"})
)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("moderation/assets/moderation.html", PageHTML)

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Moderation",
		URL:  "moderation",
		Icon: "fas fa-gavel",
	})

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/moderation"), subMux)
	web.CPMux.Handle(pat.New("/moderation/*"), subMux)

	subMux.Use(web.RequireBotMemberMW) // need the bot's role
	subMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageMessages, discordgo.PermissionEmbedLinks, discordgo.PermissionModerateMembers))

	getHandler := web.ControllerHandler(HandleModeration, "cp_moderation")
	postHandler := web.ControllerPostHandler(HandlePostModeration, getHandler, Config{})
	clearServerWarnings := web.ControllerPostHandler(HandleClearServerWarnings, getHandler, nil)

	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)
	subMux.Handle(pat.Post(""), postHandler)
	subMux.Handle(pat.Post("/"), postHandler)
	subMux.Handle(pat.Post("/clear_server_warnings"), clearServerWarnings)
}

// HandleModeration servers the moderation page itself
func HandleModeration(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["DefaultDMMessage"] = DefaultDMMessage
	templateData["DefaultTimeoutDuration"] = int(DefaultTimeoutDuration.Minutes())

	if _, ok := templateData["ModConfig"]; !ok {
		config, err := GetConfig(activeGuild.ID)
		if err != nil {
			return templateData, err
		}
		templateData["ModConfig"] = config
	}

	return templateData, nil
}

// HandlePostModeration update the settings
func HandlePostModeration(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/moderation/"

	newConfig := ctx.Value(common.ContextKeyParsedForm).(*Config)
	newConfig.DefaultMuteDuration.Valid = true
	newConfig.DefaultTimeoutDuration.Valid = true
	newConfig.DefaultBanDeleteDays.Valid = true
	templateData["ModConfig"] = newConfig

	err := newConfig.Save(activeGuild.ID)

	templateData["DefaultDMMessage"] = DefaultDMMessage

	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedSettings))
	}

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

	if rows > 0 {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyClearWarnings, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: rows}))
	}

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
