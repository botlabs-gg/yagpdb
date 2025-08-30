package autorole

import (
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/mediocregopher/radix/v3"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/autorole.html
var PageHTML string

type Form struct {
	AutoroleConfig `valid:"traverse"`
}

var _ web.SimpleConfigSaver = (*Form)(nil)

var (
	panelLogKeyUpdatedSettings = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "autorole_settings_updated", FormatString: "Updated autorole settings"})
	panelLogKeyStartedFullScan = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "autorole_full_scan", FormatString: "Started full retroactive autorole scan"})
)

func (f Form) Save(guildID int64) error {
	err := common.SetRedisJson(KeyGeneral(guildID), f.AutoroleConfig)
	if err != nil {
		return err
	}

	pubsub.EvictCacheSet(configCache, guildID)
	return nil
}

func (f Form) Name() string {
	return "Autorole"
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("autorole/assets/autorole.html", PageHTML)

	web.AddSidebarItem(web.SidebarCategoryRoles, &web.SidebarItem{
		Name: "Autorole",
		URL:  "autorole",
		Icon: "fas fa-user-plus",
	})

	muxer := goji.SubMux()

	web.CPMux.Handle(pat.New("/autorole"), muxer)
	web.CPMux.Handle(pat.New("/autorole/*"), muxer)

	muxer.Use(web.RequireBotMemberMW) // need the bot's role
	muxer.Use(web.RequirePermMW(discordgo.PermissionManageRoles))

	getHandler := web.RenderHandler(handleGetAutoroleMainPage, "cp_autorole")

	muxer.Handle(pat.Get(""), getHandler)
	muxer.Handle(pat.Get("/"), getHandler)

	muxer.Handle(pat.Post("/fullscan"), web.ControllerPostHandler(handlePostFullScan, getHandler, nil))
	muxer.Handle(pat.Post("/fullscan/cancel"), web.ControllerPostHandler(handleCancelFullScan, getHandler, nil))

	muxer.Handle(pat.Post(""), web.SimpleConfigSaverHandler(Form{}, getHandler, panelLogKeyUpdatedSettings))
	muxer.Handle(pat.Post("/"), web.SimpleConfigSaverHandler(Form{}, getHandler, panelLogKeyUpdatedSettings))
}

func handleGetAutoroleMainPage(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	general, err := GetAutoroleConfig(activeGuild.ID)
	web.CheckErr(tmpl, err, "Failed retrieving general config (contact support)", web.CtxLogger(r.Context()).Error)
	tmpl["Autorole"] = general

	var status int
	fullScanActive := false
	common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyFullScanStatus(activeGuild.ID)))
	if status > 0 {
		fullScanActive = true
		var fullScanStatus string
		switch status {
		case FullScanStarted:
			fullScanStatus = "Started"
		case FullScanIterating:
			fullScanStatus = "Iterating through the members"
		case FullScanIterationDone:
			fullScanStatus = "Iteration completed"
		case FullScanAssigningRole:
			fullScanStatus = "Assigning roles"
			var assignedRoles string
			common.RedisPool.Do(radix.Cmd(&assignedRoles, "GET", RedisKeyFullScanAssignedRoles(activeGuild.ID)))
			tmpl["AssignedRoles"] = assignedRoles
		case FullScanCancelled:
			fullScanStatus = "Cancelled"
		}
		tmpl["FullScanStatus"] = fullScanStatus
	}
	tmpl["FullScanActive"] = fullScanActive

	var proc int
	common.RedisPool.Do(radix.Cmd(&proc, "GET", KeyProcessing(activeGuild.ID)))
	tmpl["Processing"] = proc
	tmpl["ProcessingETA"] = int(proc / 60)

	return tmpl

}

func handlePostFullScan(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	if premium.ContextPremiumTier(ctx) != premium.PremiumTierPremium {
		return tmpl.AddAlerts(web.ErrorAlert("Full scan is paid premium only")), nil
	}

	err := botRestPostFullScan(activeGuild.ID)
	if err != nil {
		if err == ErrAlreadyProcessingFullGuild {
			return tmpl.AddAlerts(web.ErrorAlert("Already processing, please wait.")), nil
		}

		return tmpl, errors.WithMessage(err, "botrest")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyStartedFullScan))

	return tmpl, nil
}

func handleCancelFullScan(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	var status int64
	common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyFullScanStatus(activeGuild.ID)))
	if status == 0 {
		return tmpl.AddAlerts(web.ErrorAlert("Full scan is not active. Please refresh the page.")), nil
	}

	err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyFullScanStatus(activeGuild.ID), "10", strconv.Itoa(FullScanCancelled)))
	if err != nil {
		logger.WithError(err).Error("Failed marking Full scan as cancelled")
	}

	return tmpl, nil
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Autorole"
	templateData["SettingsPath"] = "/autorole"

	general, err := GetAutoroleConfig(ag.ID)
	if err != nil {
		return templateData, err
	}

	enabledDisabled := ""
	autoroleRole := "none"

	if role := ag.GetRole(general.Role); role != nil {
		templateData["WidgetEnabled"] = true
		enabledDisabled = web.EnabledDisabledSpanStatus(true)
		autoroleRole = html.EscapeString(role.Name)
	} else {
		templateData["WidgetDisabled"] = true
		enabledDisabled = web.EnabledDisabledSpanStatus(false)
	}

	format := `<ul>
	<li>Autorole status: %s</li>
	<li>Autorole role: <code>%s</code></li>
</ul>`

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, enabledDisabled, autoroleRole))

	return templateData, nil
}
