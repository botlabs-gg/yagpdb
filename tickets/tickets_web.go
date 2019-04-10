package tickets

import (
	"database/sql"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/tickets/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"goji.io/pat"
	"net/http"
)

type FormData struct {
	GuildID                   int64
	Enabled                   bool
	TicketsChannelCategory    int64
	TicketsTranscriptsChannel int64
	StatusChannel             int64
	TicketsUseTXTTranscripts  bool
	DownloadAttachments       bool
	ModRoles                  []int64 `valid:"role"`
	AdminRoles                []int64 `valid:"role"`
	TicketOpenMSG             string  `valid:"template,10000"`
}

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../tickets/assets/tickets_control_panel.html", "templates/plugins/tickets_control_panel.html")

	getHandler := web.ControllerHandler(p.handleGetSettings, "cp_tickets_settings")
	postHandler := web.ControllerPostHandler(p.handlePostSettings, getHandler, FormData{}, "Updated ticket settings")

	web.CPMux.Handle(pat.Get("/tickets/settings"), web.RequireGuildChannelsMiddleware(getHandler))
	web.CPMux.Handle(pat.Get("/tickets/settings/"), web.RequireGuildChannelsMiddleware(getHandler))

	web.CPMux.Handle(pat.Post("/tickets/settings"), web.RequireGuildChannelsMiddleware(postHandler))
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
		GuildID:                   activeGuild.ID,
		Enabled:                   formConfig.Enabled,
		TicketsChannelCategory:    formConfig.TicketsChannelCategory,
		TicketsTranscriptsChannel: formConfig.TicketsTranscriptsChannel,
		StatusChannel:             formConfig.StatusChannel,
		TicketsUseTXTTranscripts:  formConfig.TicketsUseTXTTranscripts,
		DownloadAttachments:       formConfig.DownloadAttachments,
		ModRoles:                  formConfig.ModRoles,
		AdminRoles:                formConfig.AdminRoles,
		TicketOpenMSG:             formConfig.TicketOpenMSG,
	}

	err := model.UpsertG(ctx, true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	return templateData, err
}
