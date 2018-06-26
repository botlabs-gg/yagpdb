package logs

import (
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"github.com/sirupsen/logrus"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type DeleteData struct {
	ID string
}

type GeneralFormData struct {
	UsernameLoggingEnabled       bool
	NicknameLoggingEnabled       bool
	ManageMessagesCanViewDeleted bool
	EveryoneCanViewDeleted       bool
	BlacklistedChannels          []string
}

func (lp *Plugin) InitWeb() {
	tmplPathSettings := "templates/plugins/logs_control_panel.html"
	tmplPathView := "templates/plugins/logs_view.html"
	if common.Testing {
		tmplPathSettings = "../../logs/assets/logs_control_panel.html"
		tmplPathView = "../../logs/assets/logs_view.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings, tmplPathView))

	web.ServerPublicMux.Handle(pat.Get("/logs/:id"), web.RenderHandler(HandleLogsHTML, "public_server_logs"))
	web.ServerPublicMux.Handle(pat.Get("/logs/:id/"), web.RenderHandler(HandleLogsHTML, "public_server_logs"))

	logCPMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/logging"), logCPMux)
	web.CPMux.Handle(pat.New("/logging/*"), logCPMux)

	logCPMux.Use(web.RequireGuildChannelsMiddleware)

	cpGetHandler := web.ControllerHandler(HandleLogsCP, "cp_logging")
	logCPMux.Handle(pat.Get("/"), cpGetHandler)
	logCPMux.Handle(pat.Get(""), cpGetHandler)

	saveHandler := web.ControllerPostHandler(HandleLogsCPSaveGeneral, cpGetHandler, GeneralFormData{}, "Updated logging config")
	fullDeleteHandler := web.ControllerPostHandler(HandleLogsCPDelete, cpGetHandler, DeleteData{}, "Deleted a channel log")
	msgDeleteHandler := web.APIHandler(HandleDeleteMessageJson)

	logCPMux.Handle(pat.Post("/"), saveHandler)
	logCPMux.Handle(pat.Post(""), saveHandler)

	logCPMux.Handle(pat.Post("/fulldelete"), fullDeleteHandler)
	logCPMux.Handle(pat.Post("/msgdelete"), msgDeleteHandler)
}

func HandleLogsCP(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	beforeID := 0
	beforeStr := r.URL.Query().Get("before")
	if beforeStr != "" {
		beforeId64, err := strconv.ParseInt(beforeStr, 10, 32)
		if err != nil {
			tmpl.AddAlerts(web.ErrorAlert("Failed parsing before id"))
		}
		beforeID = int(beforeId64)
	} else {
		tmpl["FirstPage"] = true
	}

	afterID := 0
	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		id64, err := strconv.ParseInt(afterStr, 10, 32)
		if err != nil {
			tmpl.AddAlerts(web.ErrorAlert("Failed parsing before id"))
		}
		afterID = int(id64)
		tmpl["FirstPage"] = false
	}

	serverLogs, err := GetGuilLogs(g.ID, beforeID, afterID, 20)
	web.CheckErr(tmpl, err, "Failed retrieving logs", web.CtxLogger(ctx).Error)
	if err == nil {
		tmpl["Logs"] = serverLogs
		if len(serverLogs) > 0 {
			tmpl["Oldest"] = serverLogs[len(serverLogs)-1].ID
			tmpl["Newest"] = serverLogs[0].ID
		}
	}

	general, err := GetConfig(g.ID)
	if err != nil {
		return nil, err
	}
	tmpl["Config"] = general

	return tmpl, nil
}

func HandleLogsCPSaveGeneral(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	form := ctx.Value(common.ContextKeyParsedForm).(*GeneralFormData)

	config := &GuildLoggingConfig{
		GuildConfigModel: configstore.GuildConfigModel{
			GuildID: g.ID,
		},

		NicknameLoggingEnabled:       form.NicknameLoggingEnabled,
		UsernameLoggingEnabled:       form.UsernameLoggingEnabled,
		BlacklistedChannels:          strings.Join(form.BlacklistedChannels, ","),
		EveryoneCanViewDeleted:       form.EveryoneCanViewDeleted,
		ManageMessagesCanViewDeleted: form.ManageMessagesCanViewDeleted,
	}

	err := configstore.SQL.SetGuildConfig(ctx, config)
	return tmpl, err
}

func HandleLogsCPDelete(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	data := ctx.Value(common.ContextKeyParsedForm).(*DeleteData)
	if data.ID == "" {
		return tmpl, errors.New("ID is blank!")
	}

	result := common.GORM.Where("id = ? AND guild_id = ?", data.ID, g.ID).Delete(MessageLog{})
	if result.Error != nil {
		return tmpl, result.Error
	}

	if result.RowsAffected < 1 {
		tmpl.AddAlerts(web.ErrorAlert("Ahhhhh did nothing??"))
		return tmpl, nil
	}
	logrus.Println(result.RowsAffected)

	err := common.GORM.Where("message_log_id = ?", data.ID).Delete(Message{}).Error
	return tmpl, err
}

func HandleLogsHTML(w http.ResponseWriter, r *http.Request) interface{} {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	idString := pat.Param(r, "id")

	parsed, err := strconv.ParseInt(idString, 10, 64)
	if web.CheckErr(tmpl, err, "Thats's not a real log id", nil) {
		return tmpl
	}

	config, err := GetConfig(g.ID)
	if web.CheckErr(tmpl, err, "Error retrieving config for this server", web.CtxLogger(r.Context()).Error) {
		return tmpl
	}

	canViewDeleted := web.IsAdminCtx(r.Context())
	if config.EveryoneCanViewDeleted {
		canViewDeleted = true
	} else if config.ManageMessagesCanViewDeleted && !canViewDeleted {
		canViewDeleted = web.HasPermissionCTX(r.Context(), discordgo.PermissionManageMessages)
	}
	tmpl["CanViewDeleted"] = canViewDeleted

	msgLogs, err := GetChannelLogs(parsed)
	if web.CheckErr(tmpl, err, "Failed retrieving message logs", web.CtxLogger(r.Context()).Error) {
		return tmpl
	}

	if msgLogs.GuildID != discordgo.StrID(g.ID) {
		return tmpl.AddAlerts(web.ErrorAlert("Couldn't find the logs im so sorry please dont hurt me i have a family D:"))
	}

	const TimeFormat = "2006 Jan 02 15:04"
	for k, v := range msgLogs.Messages {
		parsed, err := discordgo.Timestamp(v.Timestamp).Parse()
		if err != nil {
			logrus.WithError(err).Error("Failed parsing logged message timestamp")
			continue
		}
		ts := parsed.UTC().Format(TimeFormat)
		msgLogs.Messages[k].Timestamp = ts
	}

	tmpl["Logs"] = msgLogs
	return tmpl
}

func HandleDeleteMessageJson(w http.ResponseWriter, r *http.Request) interface{} {
	_, g, _ := web.GetBaseCPContextData(r.Context())

	logsId := r.FormValue("LogID")
	msgID := r.FormValue("MessageID")

	if logsId == "" || msgID == "" {
		return web.NewPublicError("Empty id")
	}

	var logContainer MessageLog
	err := common.GORM.Where("id = ?", logsId).First(&logContainer).Error
	if err != nil {
		return err
	}

	if logContainer.GuildID != discordgo.StrID(g.ID) {
		return err
	}

	err = common.GORM.Model(&Message{}).Where("message_log_id = ? AND id = ?", logsId, msgID).Update("deleted", true).Error
	user := r.Context().Value(common.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, g.ID, "Deleted a message from log #"+logsId)
	return err
}
