package tickets

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"

	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/cplogs"
	"github.com/jonas747/yagpdb/tickets/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"goji.io/pat"
)

type FormData struct {
	GuildID                            int64
	Enabled                            bool
	TicketsChannelCategory             int64 `valid:"channel,true"`
	TicketsTranscriptsChannel          int64 `valid:"channel,true"`
	TicketsTranscriptsChannelAdminOnly int64 `valid:"channel,true"`
	StatusChannel                      int64 `valid:"channel,true"`
	TicketsUseTXTTranscripts           bool
	DownloadAttachments                bool
	ModRoles                           []int64 `valid:"role"`
	AdminRoles                         []int64 `valid:"role"`
	TicketOpenMSG                      string  `valid:"template,10000"`
}

var panelLogKey = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "tickets_updated_settings", FormatString: "Updated ticket settings"})

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../tickets/assets/tickets_control_panel.html", "templates/plugins/tickets_control_panel.html")

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Ticket System",
		URL:  "tickets/settings",
		Icon: "fas fa-ticket-alt",
	})

	getHandler := web.ControllerHandler(p.handleGetSettings, "cp_tickets_settings")
	postHandler := web.ControllerPostHandler(p.handlePostSettings, getHandler, FormData{})

	web.CPMux.Handle(pat.Get("/tickets/settings"), getHandler)
	web.CPMux.Handle(pat.Get("/tickets/settings/"), getHandler)

	web.CPMux.Handle(pat.Post("/tickets/settings"), postHandler)
}

func (p *Plugin) handleGetSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	settings, err := models.FindTicketConfigG(ctx, activeGuild.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			return templateData, err
		}

		// return standard config
		settings = &models.TicketConfig{}
	}

	templateData["DefaultTicketMessage"] = DefaultTicketMsg
	templateData["PluginSettings"] = settings

	return templateData, nil
}

func (p *Plugin) handlePostSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	formConfig := ctx.Value(common.ContextKeyParsedForm).(*FormData)

	model := &models.TicketConfig{
		GuildID:                            activeGuild.ID,
		Enabled:                            formConfig.Enabled,
		TicketsChannelCategory:             formConfig.TicketsChannelCategory,
		TicketsTranscriptsChannel:          formConfig.TicketsTranscriptsChannel,
		TicketsTranscriptsChannelAdminOnly: formConfig.TicketsTranscriptsChannelAdminOnly,
		StatusChannel:                      formConfig.StatusChannel,
		TicketsUseTXTTranscripts:           formConfig.TicketsUseTXTTranscripts,
		DownloadAttachments:                formConfig.DownloadAttachments,
		ModRoles:                           formConfig.ModRoles,
		AdminRoles:                         formConfig.AdminRoles,
		TicketOpenMSG:                      formConfig.TicketOpenMSG,
	}

	err := model.UpsertG(ctx, true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKey))
	}
	return templateData, err
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	settings, err := models.FindTicketConfigG(r.Context(), activeGuild.ID)
	if err != nil && err != sql.ErrNoRows {
		return templateData, err
	}

	enabled := false
	if settings != nil {
		enabled = true
	}

	templateData["WidgetTitle"] = "Tickets"
	templateData["SettingsPath"] = "/tickets/settings"
	if enabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<ul>
	<li>Tickets enabled: %s</li>
 </ul>`

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, web.EnabledDisabledSpanStatus(enabled)))

	return templateData, nil
}
