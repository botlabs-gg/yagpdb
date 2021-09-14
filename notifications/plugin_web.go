package notifications

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/common/configstore"
	"github.com/botlabs-gg/yagpdb/common/cplogs"
	"github.com/botlabs-gg/yagpdb/web"
	"github.com/jonas747/discordgo/v2"
	"goji.io/pat"
)

//go:embed assets/notifications_general.html
var PageHTML string

var panelLogKey = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "notifications_settings", FormatString: "Updated server notification settings"})

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("notifications/assets/notifications_general.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "General",
		URL:  "notifications/general",
		Icon: "fas fa-bell",
	})

	getHandler := web.RenderHandler(HandleNotificationsGet, "cp_notifications_general")
	postHandler := web.ControllerPostHandler(HandleNotificationsPost, getHandler, Config{})

	web.CPMux.Handle(pat.Get("/notifications/general"), getHandler)
	web.CPMux.Handle(pat.Get("/notifications/general/"), getHandler)

	web.CPMux.Handle(pat.Post("/notifications/general"), postHandler)
	web.CPMux.Handle(pat.Post("/notifications/general/"), postHandler)

	web.ServerPubliAPIMux.Handle(pat.Get("/welcome_messages"), web.RequireServerAdminMiddleware(web.APIHandler(HandleNotificationsGetAPI)))
}

func HandleNotificationsGet(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	formConfig, ok := ctx.Value(common.ContextKeyParsedForm).(*Config)
	if ok {
		templateData["NotifyConfig"] = formConfig
	} else {
		conf, err := GetConfig(activeGuild.ID)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).Error("failed retrieving config")
		}

		templateData["NotifyConfig"] = conf
	}

	return templateData
}

func HandleNotificationsPost(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/notifications/general/"

	newConfig := ctx.Value(common.ContextKeyParsedForm).(*Config)

	newConfig.GuildID = activeGuild.ID

	err := configstore.SQL.SetGuildConfig(ctx, newConfig)
	if err != nil {
		return templateData, nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKey))

	return templateData, nil
}

type ConfigResponse struct {
	GuildID int64 `json:"guild_id,string"`

	JoinServerEnabled bool     `json:"join_server_enabled"`
	JoinServerChannel int64    `json:"join_server_channel,string"`
	JoinServerMsgs    []string `json:"join_server_msg"`
	JoinDMEnabled     bool     `json:"join_dm_enabled"`
	JoinDMMsg         string   `json:"join_dm_msg"`

	LeaveEnabled bool     `json:"leave_enabled"`
	LeaveChannel int64    `json:"leave_channel,string"`
	LeaveMsgs    []string `json:"leave_msg"`

	CensorInvites bool `json:"censor_invites"`
}

func HandleNotificationsGetAPI(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild := web.ContextGuild(r.Context())
	conf, err := GetConfig(activeGuild.ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("failed retrieving config")
		return err
	}

	return ConfigResponse{
		GuildID:           conf.GuildID,
		JoinServerEnabled: conf.JoinServerEnabled,
		JoinServerChannel: conf.JoinServerChannelInt(),
		JoinServerMsgs:    conf.JoinServerMsgs,
		JoinDMEnabled:     conf.JoinDMEnabled,
		JoinDMMsg:         conf.JoinDMMsg,
		LeaveEnabled:      conf.LeaveEnabled,
		LeaveChannel:      conf.LeaveChannelInt(),
		LeaveMsgs:         conf.LeaveMsgs,
		CensorInvites:     conf.CensorInvites,
	}
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "General notifications"
	templateData["SettingsPath"] = "/notifications/general"

	config, err := GetConfig(ag.ID)
	if err != nil {
		return templateData, err
	}

	format := `<ul>
	<li>Join Server message: %s</li>
	<li>Join DM message: %s</li>
	<li>Leave message: %s</li>
	<li>Topic change message: %s</li>
</ul>`

	if config.JoinServerEnabled || config.JoinDMEnabled || config.LeaveEnabled || config.TopicEnabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format,
		web.EnabledDisabledSpanStatus(config.JoinServerEnabled), web.EnabledDisabledSpanStatus(config.JoinDMEnabled),
		web.EnabledDisabledSpanStatus(config.LeaveEnabled), web.EnabledDisabledSpanStatus(config.TopicEnabled)))

	return templateData, nil
}

func enabledDisabled(b bool) string {
	if b {
		return "enabled"
	}

	return "disabled"
}
