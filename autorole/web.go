package autorole

import (
	"fmt"
	"html"
	"html/template"
	"net/http"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
	"github.com/mediocregopher/radix/v3"
	"goji.io"
	"goji.io/pat"
)

type Form struct {
	GeneralConfig `valid:"traverse"`
}

var _ web.SimpleConfigSaver = (*Form)(nil)

func (f Form) Save(guildID int64) error {
	pubsub.Publish("autorole_stop_processing", guildID, nil)

	err := common.SetRedisJson(KeyGeneral(guildID), f.GeneralConfig)
	if err != nil {
		return err
	}

	return nil
}

func (f Form) Name() string {
	return "Autorole"
}

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../autorole/assets/autorole.html", "templates/plugins/autorole.html")

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
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

	muxer.Handle(pat.Post("/fullscan"), web.ControllerPostHandler(handlePostFullScan, getHandler, nil, "Triggered a full autorole scan"))

	muxer.Handle(pat.Post(""), web.SimpleConfigSaverHandler(Form{}, getHandler))
	muxer.Handle(pat.Post("/"), web.SimpleConfigSaverHandler(Form{}, getHandler))
}

func handleGetAutoroleMainPage(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	general, err := GetGeneralConfig(activeGuild.ID)
	web.CheckErr(tmpl, err, "Failed retrieving general config (contact support)", web.CtxLogger(r.Context()).Error)
	tmpl["Autorole"] = general

	var proc int
	common.RedisPool.Do(radix.Cmd(&proc, "GET", KeyProcessing(activeGuild.ID)))
	tmpl["Processing"] = proc
	tmpl["ProcessingETA"] = int(proc / 60)

	fullScanActive := WorkingOnFullScan(activeGuild.ID)
	tmpl["FullScanActive"] = fullScanActive

	return tmpl

}

func handlePostFullScan(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	err := botRestPostFullScan(activeGuild.ID)
	if err != nil {
		if err == ErrAlreadyProcessingFullGuild {
			return tmpl.AddAlerts(web.ErrorAlert("Already processing, please wait.")), nil
		}

		return tmpl, errors.WithMessage(err, "botrest")
	}

	return tmpl, nil
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Autorole"
	templateData["SettingsPath"] = "/autorole"

	general, err := GetGeneralConfig(ag.ID)
	if err != nil {
		return templateData, err
	}

	enabledDisabled := ""
	autoroleRole := "none"

	if role := ag.Role(general.Role); role != nil {
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
