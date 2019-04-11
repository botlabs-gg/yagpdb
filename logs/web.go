package logs

import (
	"errors"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strconv"
	"strings"
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

	saveHandler := web.ControllerPostHandler(HandleLogsCPSaveGeneral, cpGetHandler, ConfigFormData{}, "Updated logging config")
	fullDeleteHandler := web.ControllerPostHandler(HandleLogsCPDelete, cpGetHandler, DeleteData{}, "Deleted a channel log")
	msgDeleteHandler := web.APIHandler(HandleDeleteMessageJson)

	logCPMux.Handle(pat.Post("/"), saveHandler)
	logCPMux.Handle(pat.Post(""), saveHandler)

	logCPMux.Handle(pat.Post("/fulldelete"), fullDeleteHandler)
	logCPMux.Handle(pat.Post("/msgdelete"), msgDeleteHandler)
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

	general, err := GetConfig(ctx, g.ID)
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
	return tmpl, err
}

func HandleLogsCPDelete(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	data := ctx.Value(common.ContextKeyParsedForm).(*DeleteData)
	if data.ID == 0 {
		return tmpl, errors.New("ID is blank!")
	}

	_, err := models.MessageLogs(models.MessageLogWhere.ID.EQ(int(data.ID)),
		models.MessageLogWhere.GuildID.EQ(null.StringFrom(discordgo.StrID(g.ID)))).DeleteAll(r.Context(), common.PQ)

	if err != nil {
		return tmpl, err
	}

	// for legacy setups
	_, err = models.Messages(models.MessageWhere.MessageLogID.EQ(null.IntFrom(int(data.ID)))).DeleteAll(ctx, common.PQ)
	return tmpl, err
}

func HandleLogsHTML(w http.ResponseWriter, r *http.Request) interface{} {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	idString := pat.Param(r, "id")

	parsed, err := strconv.ParseInt(idString, 10, 64)
	if web.CheckErr(tmpl, err, "Thats's not a real log id", nil) {
		return tmpl
	}

	config, err := GetConfig(r.Context(), g.ID)
	if web.CheckErr(tmpl, err, "Error retrieving config for this server", web.CtxLogger(r.Context()).Error) {
		return tmpl
	}

	isAdmin := web.IsAdminRequest(r.Context(), r)

	// check if were allowed access to logs on this server
	if !isAdmin && len(config.MessageLogsAllowedRoles) > 0 {
		member := web.ContextMember(r.Context())
		if member == nil {
			return tmpl.AddAlerts(web.ErrorAlert("This server has restricted log access to certain roles, either you're not logged in or not on this server."))
		}

		if !common.ContainsInt64SliceOneOf(member.Roles, config.MessageLogsAllowedRoles) {
			return tmpl.AddAlerts(web.ErrorAlert("This server has restricted log access to certain roles, you don't have any of them."))
		}
	}

	// check if were allowed to view deleted messages
	canViewDeleted := isAdmin
	if config.EveryoneCanViewDeleted.Bool {
		canViewDeleted = true
	} else if config.ManageMessagesCanViewDeleted.Bool && !canViewDeleted {
		canViewDeleted = web.HasPermissionCTX(r.Context(), discordgo.PermissionManageMessages)
	}

	tmpl["CanViewDeleted"] = canViewDeleted

	// retrieve logs
	msgLogs, err := GetChannelLogs(r.Context(), parsed, g.ID)
	if web.CheckErr(tmpl, err, "Failed retrieving message logs", web.CtxLogger(r.Context()).Error) {
		return tmpl
	}

	if msgLogs.GuildID.String != discordgo.StrID(g.ID) {
		return tmpl.AddAlerts(web.ErrorAlert("Couldn't find the logs im so sorry please dont hurt me i have a family D:"))
	}

	// Fetch the role colors if possible
	users := make([]int64, 0, 50)
	for _, v := range msgLogs.R.Messages {
		parsedAuthor, _ := strconv.ParseInt(v.AuthorID.String, 10, 64)
		if !common.ContainsInt64Slice(users, parsedAuthor) {
			users = append(users, parsedAuthor)
		}
	}

	roleColors, _ := botrest.GetMemberColors(g.ID, users...)

	extraColors := make([]string, len(msgLogs.R.Messages))

	const TimeFormat = "2006 Jan 02 15:04"
	for k, v := range msgLogs.R.Messages {
		parsed, err := discordgo.Timestamp(v.Timestamp.String).Parse()
		if err != nil {
			logrus.WithError(err).Error("Failed parsing logged message timestamp")
			continue
		}
		ts := parsed.UTC().Format(TimeFormat)
		msgLogs.R.Messages[k].Timestamp = null.StringFrom(ts)

		if roleColors != nil {
			if c, ok := roleColors[v.AuthorID.String]; ok {
				extraColors[k] = strconv.FormatInt(int64(c), 16)
			}
		}
	}

	tmpl["MessageColors"] = extraColors
	tmpl["Logs"] = msgLogs
	return tmpl
}

func HandleDeleteMessageJson(w http.ResponseWriter, r *http.Request) interface{} {
	g, _ := web.GetBaseCPContextData(r.Context())

	logsId := r.FormValue("LogID")
	msgID := r.FormValue("MessageID")

	if logsId == "" || msgID == "" {
		return web.NewPublicError("Empty id")
	}

	parsedLogsID, _ := strconv.ParseInt(logsId, 10, 64)
	_, err := models.MessageLogs(
		models.MessageLogWhere.ID.EQ(int(parsedLogsID)),
		models.MessageLogWhere.GuildID.EQ(null.StringFrom(discordgo.StrID(g.ID)))).OneG(r.Context())

	if err != nil {
		return err
	}

	parsedMsgID, _ := strconv.ParseInt(msgID, 10, 64)
	_, err = models.Messages(
		models.MessageWhere.ID.EQ(int(parsedMsgID)),
		models.MessageWhere.MessageLogID.EQ(null.IntFrom(int(parsedLogsID)))).UpdateAllG(
		r.Context(), models.M{"deleted": true})

	if err != nil {
		return err
	}

	user := r.Context().Value(common.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, g.ID, "Deleted a message from log #"+logsId)
	return err
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Logging"
	templateData["SettingsPath"] = "/logging/"

	config, err := GetConfig(r.Context(), activeGuild.ID)
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
	<li>Nickname loggin: %s</li>
	<li>Blacklisted channels from creating message logs: <code>%d</code></li>
</ul>`

	templateData["WidgetEnabled"] = true

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, web.EnabledDisabledSpanStatus(config.UsernameLoggingEnabled.Bool),
		web.EnabledDisabledSpanStatus(config.NicknameLoggingEnabled.Bool), nBlacklistedChannels))

	return templateData, nil
}
