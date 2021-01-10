package logs

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/cplogs"
	"github.com/jonas747/yagpdb/logs/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"goji.io"
	"goji.io/pat"
)

var AuthorColors = []string{
	"7c7cff", // blue-ish
	"529fb7", // lighter blue
	"4aa085", // dark-green
	"7ea04a", // lighter green
	"a0824a", // brown
	"a04a4a", // red
	"a04a89", // purple?
}

type DeleteData struct {
	ID int64
}

type ConfigFormData struct {
	UsernameLoggingEnabled       bool
	NicknameLoggingEnabled       bool
	ManageMessagesCanViewDeleted bool
	EveryoneCanViewDeleted       bool
	BlacklistedChannels          []string
	MessageLogsAllowedRoles      []int64
}

var (
	panelLogKeyUpdatedSettings   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "logs_settings_updated", FormatString: "Updated logging settings"})
	panelLogKeyDeletedMessageLog = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "logs_deleted_message_log", FormatString: "Deleted a message log: %d"})
	panelLogKeyDeletedMessage    = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "logs_deleted_message", FormatString: "Deleted a message from a message log: %d"})
)

func (lp *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../logs/assets/logs_control_panel.html", "templates/plugins/logs_control_panel.html")
	web.LoadHTMLTemplate("../../logs/assets/logs_view.html", "templates/plugins/logs_view.html")

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Logging",
		URL:  "logging/",
		Icon: "fas fa-database",
	})

	web.ServerPublicMux.Handle(pat.Get("/logs/:id"), web.RenderHandler(LogFetchMW(HandleLogsHTML, true), "public_server_logs"))
	web.ServerPublicMux.Handle(pat.Get("/logs/:id/"), web.RenderHandler(LogFetchMW(HandleLogsHTML, true), "public_server_logs"))

	web.ServerPublicMux.Handle(pat.Get("/log/:id"), web.RenderHandler(LogFetchMW(HandleLogsHTML, false), "public_server_logs"))
	web.ServerPublicMux.Handle(pat.Get("/log/:id/"), web.RenderHandler(LogFetchMW(HandleLogsHTML, false), "public_server_logs"))

	logCPMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/logging"), logCPMux)
	web.CPMux.Handle(pat.New("/logging/*"), logCPMux)

	cpGetHandler := web.ControllerHandler(HandleLogsCP, "cp_logging")
	logCPMux.Handle(pat.Get("/"), cpGetHandler)
	logCPMux.Handle(pat.Get(""), cpGetHandler)

	saveHandler := web.ControllerPostHandler(HandleLogsCPSaveGeneral, cpGetHandler, ConfigFormData{})
	fullDeleteHandler := web.ControllerPostHandler(HandleLogsCPDelete, cpGetHandler, DeleteData{})
	msgDeleteHandler := web.APIHandler(HandleDeleteMessageJson)

	logCPMux.Handle(pat.Post("/"), saveHandler)
	logCPMux.Handle(pat.Post(""), saveHandler)

	logCPMux.Handle(pat.Post("/fulldelete2"), fullDeleteHandler)
	logCPMux.Handle(pat.Post("/msgdelete2"), msgDeleteHandler)
}

func HandleLogsCP(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

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

	serverLogs, err := GetGuilLogs(ctx, g.ID, beforeID, afterID, 20)
	web.CheckErr(tmpl, err, "Failed retrieving logs", web.CtxLogger(ctx).Error)
	if err == nil {
		tmpl["Logs"] = serverLogs
		if len(serverLogs) > 0 {
			tmpl["Oldest"] = serverLogs[len(serverLogs)-1].ID
			tmpl["Newest"] = serverLogs[0].ID
		}
	}

	general, err := GetConfig(common.PQ, ctx, g.ID)
	if err != nil {
		return nil, err
	}
	tmpl["Config"] = general

	// dealing with legacy code is a pain, gah
	// so way back i didn't know about arrays in postgres, so i made the blacklisted channels field a single TEXT field, with a comma seperator
	blacklistedChannels := make([]int64, 0, 10)
	split := strings.Split(general.BlacklistedChannels.String, ",")
	for _, v := range split {
		i, err := strconv.ParseInt(v, 10, 64)
		if i != 0 && err == nil {
			blacklistedChannels = append(blacklistedChannels, i)
		}
	}
	tmpl["ConfBlacklistedChannels"] = blacklistedChannels

	return tmpl, nil
}

func HandleLogsCPSaveGeneral(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	form := ctx.Value(common.ContextKeyParsedForm).(*ConfigFormData)

	config := &models.GuildLoggingConfig{
		GuildID: g.ID,

		NicknameLoggingEnabled:       null.BoolFrom(form.NicknameLoggingEnabled),
		UsernameLoggingEnabled:       null.BoolFrom(form.UsernameLoggingEnabled),
		BlacklistedChannels:          null.StringFrom(strings.Join(form.BlacklistedChannels, ",")),
		EveryoneCanViewDeleted:       null.BoolFrom(form.EveryoneCanViewDeleted),
		ManageMessagesCanViewDeleted: null.BoolFrom(form.ManageMessagesCanViewDeleted),
		MessageLogsAllowedRoles:      form.MessageLogsAllowedRoles,
	}

	err := config.UpsertG(ctx, true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	if err == nil {
		bot.EvictGSCache(g.ID, CacheKeyConfig)
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedSettings))

	}
	return tmpl, err
}

func HandleLogsCPDelete(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	data := ctx.Value(common.ContextKeyParsedForm).(*DeleteData)
	if data.ID == 0 {
		return tmpl, errors.New("ID is blank!")
	}

	_, err := models.MessageLogs2s(
		models.MessageLogs2Where.ID.EQ(int(data.ID)),
		models.MessageLogs2Where.GuildID.EQ(g.ID),
	).DeleteAll(r.Context(), common.PQ)

	if err != nil {
		return tmpl, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyDeletedMessageLog, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: data.ID}))

	// for legacy setups
	// _, err = models.Messages(models.MessageWhere.MessageLogID.EQ(null.IntFrom(int(data.ID)))).DeleteAll(ctx, common.PQ)
	return tmpl, err
}

func CheckCanAccessLogs(w http.ResponseWriter, r *http.Request, config *models.GuildLoggingConfig) bool {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	isAdmin, _ := web.IsAdminRequest(r.Context(), r)

	// check if were allowed access to logs on this server
	if isAdmin || len(config.MessageLogsAllowedRoles) < 1 {
		return true
	}

	member := web.ContextMember(r.Context())
	if member == nil {
		tmpl.AddAlerts(web.ErrorAlert("This server has restricted log access to certain roles, either you're not logged in or not on this server."))
		return false
	}

	if !common.ContainsInt64SliceOneOf(member.Roles, config.MessageLogsAllowedRoles) {
		tmpl.AddAlerts(web.ErrorAlert("This server has restricted log access to certain roles, you don't have any of them."))
		return false
	}

	return true
}

type ctxKey int

const (
	ctxKeyLogs ctxKey = iota
	ctxKeyMessages
	ctxKeyConfig
)

func LogFetchMW(inner web.CustomHandlerFunc, legacy bool) web.CustomHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) interface{} {
		g, tmpl := web.GetBaseCPContextData(r.Context())

		idString := pat.Param(r, "id")

		parsed, err := strconv.ParseInt(idString, 10, 64)
		if web.CheckErr(tmpl, err, "Thats's not a real log id", nil) {
			return tmpl
		}

		config, err := GetConfig(common.PQ, r.Context(), g.ID)
		if web.CheckErr(tmpl, err, "Error retrieving config for this server", web.CtxLogger(r.Context()).Error) {
			return tmpl
		}

		if !CheckCanAccessLogs(w, r, config) {
			return tmpl
		}

		sm := SearchModeLegacy
		if !legacy {
			sm = SearchModeNew
		}

		// retrieve logs
		msgLogs, messages, err := GetChannelLogs(r.Context(), parsed, g.ID, sm)
		if web.CheckErr(tmpl, err, "Failed retrieving message logs", web.CtxLogger(r.Context()).Error) {
			return tmpl
		}

		if msgLogs.GuildID != g.ID {
			return tmpl.AddAlerts(web.ErrorAlert("Couldn't find the logs im so sorry please dont hurt me i have a family D:"))
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxKeyLogs, msgLogs)
		ctx = context.WithValue(ctx, ctxKeyMessages, messages)
		ctx = context.WithValue(ctx, ctxKeyConfig, config)

		return inner(w, r.WithContext(ctx))
	}
}

type MessageView struct {
	Model *models.Messages2

	Color     string
	Timestamp string
}

func HandleLogsHTML(w http.ResponseWriter, r *http.Request) interface{} {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	logs := r.Context().Value(ctxKeyLogs).(*models.MessageLogs2)
	messages := r.Context().Value(ctxKeyMessages).([]*models.Messages2)
	config := r.Context().Value(ctxKeyConfig).(*models.GuildLoggingConfig)

	// check if were allowed to view deleted messages
	isAdmin, _ := web.IsAdminRequest(r.Context(), r)

	var canViewDeleted = false
	if isAdmin && !web.GetIsReadOnly(r.Context()) {
		canViewDeleted = true
	} else if config.EveryoneCanViewDeleted.Bool {
		canViewDeleted = true
	} else if config.ManageMessagesCanViewDeleted.Bool && !canViewDeleted {
		canViewDeleted = web.HasPermissionCTX(r.Context(), discordgo.PermissionManageMessages)
	}

	tmpl["CanViewDeleted"] = canViewDeleted

	// Convert into views with formatted dates and colors
	const TimeFormat = "2006 Jan 02 15:04"
	messageViews := make([]*MessageView, len(messages))
	for i, _ := range messageViews {
		m := messages[i]
		v := &MessageView{
			Model:     m,
			Timestamp: m.CreatedAt.Format(TimeFormat),
		}
		messageViews[i] = v
	}

	SetMessageLogsColors(g.ID, messageViews)

	tmpl["Logs"] = logs
	tmpl["Messages"] = messageViews

	return tmpl
}

func SetMessageLogsColors(guildID int64, views []*MessageView) {
	users := make([]int64, 0, 50)

	for _, v := range views {
		if !common.ContainsInt64Slice(users, v.Model.AuthorID) {
			users = append(users, v.Model.AuthorID)
		}
	}

	roleColors, _ := botrest.GetMemberColors(guildID, users...)
	if roleColors == nil {
		return
	}

	for _, v := range views {
		strAuthorID := strconv.FormatInt(v.Model.AuthorID, 10)
		color := roleColors[strAuthorID]
		if color != 0 {
			v.Color = strconv.FormatInt(int64(color), 16)
		}
	}
}

func HandleDeleteMessageJson(w http.ResponseWriter, r *http.Request) interface{} {
	g, _ := web.GetBaseCPContextData(r.Context())

	logsId := r.FormValue("LogID")
	msgID := r.FormValue("MessageID")

	if logsId == "" || msgID == "" {
		return web.NewPublicError("Empty id")
	}

	parsedLogsID, _ := strconv.ParseInt(logsId, 10, 64)
	_, err := models.MessageLogs2s(
		models.MessageLogs2Where.ID.EQ(int(parsedLogsID)),
		models.MessageLogs2Where.GuildID.EQ(g.ID),
	).OneG(r.Context())

	if err != nil {
		return err
	}

	parsedMsgID, _ := strconv.ParseInt(msgID, 10, 64)

	_, err = models.Messages2s(
		models.Messages2Where.ID.EQ(parsedMsgID),
		models.Messages2Where.GuildID.EQ(g.ID)).UpdateAllG(
		r.Context(), models.M{"deleted": true})

	if err != nil {
		return err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyDeletedMessage, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: parsedMsgID}))

	return err
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Logging"
	templateData["SettingsPath"] = "/logging/"

	config, err := GetConfig(common.PQ, r.Context(), activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	nBlacklistedChannels := 0

	if len(config.BlacklistedChannels.String) > 0 {
		split := strings.Split(config.BlacklistedChannels.String, ",")
		nBlacklistedChannels = len(split)
	}

	format := `<ul>
	<li>Username logging: %s</li>
	<li>Nickname logging: %s</li>
	<li>Blacklisted channels from creating message logs: <code>%d</code></li>
</ul>`

	templateData["WidgetEnabled"] = true

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, web.EnabledDisabledSpanStatus(config.UsernameLoggingEnabled.Bool),
		web.EnabledDisabledSpanStatus(config.NicknameLoggingEnabled.Bool), nBlacklistedChannels))

	return templateData, nil
}
