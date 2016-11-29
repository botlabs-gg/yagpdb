package logs

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

type DeleteData struct {
	ID string
}

func (lp *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/logging.html"))
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/public_channel_logs.html"))

	web.ServerPublicMux.HandleC(pat.Get("/logs/:id"), web.RenderHandler(HandleLogsHTML, "public_server_logs"))
	web.ServerPublicMux.HandleC(pat.Get("/logs/:id/"), web.RenderHandler(HandleLogsHTML, "public_server_logs"))

	logCPMux := goji.SubMux()
	web.CPMux.HandleC(pat.New("/logging"), logCPMux)
	web.CPMux.HandleC(pat.New("/logging/*"), logCPMux)

	cpGetHandler := web.ControllerHandler(HandleLogsCP, "cp_logging")
	logCPMux.HandleC(pat.Get("/"), cpGetHandler)
	logCPMux.HandleC(pat.Get(""), cpGetHandler)

	saveHandler := web.ControllerPostHandler(HandleLogsCPSaveGeneral, cpGetHandler, GuildLoggingConfig{}, "Updated logging config")
	deleteHandler := web.ControllerPostHandler(HandleLogsCPDelete, cpGetHandler, DeleteData{}, "Deleted a channel log")

	logCPMux.HandleC(pat.Post("/"), saveHandler)
	logCPMux.HandleC(pat.Post(""), saveHandler)

	logCPMux.HandleC(pat.Post("/delete"), deleteHandler)
}

func HandleLogsCP(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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
	web.CheckErr(tmpl, err, "Failed retrieving logs", logrus.Error)
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

func HandleLogsCPSaveGeneral(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	config := ctx.Value(common.ContextKeyParsedForm).(*GuildLoggingConfig)
	parsed, _ := strconv.ParseInt(g.ID, 10, 64)
	config.GuildID = parsed

	err := configstore.SQL.SetGuildConfig(ctx, config)
	return tmpl, err
}

func HandleLogsCPDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	data := ctx.Value(common.ContextKeyParsedForm).(*DeleteData)
	if data.ID == "" {
		return tmpl, errors.New("ID is blank!")
	}

	result := common.SQL.Where("id = ? AND guild_id = ?", data.ID, g.ID).Delete(MessageLog{})
	if result.Error != nil {
		return tmpl, result.Error
	}

	if result.RowsAffected < 1 {
		tmpl.AddAlerts(web.ErrorAlert("Ahhhhh did nothing??"))
		return tmpl, nil
	}
	logrus.Println(result.RowsAffected)

	err := common.SQL.Where("message_log_id = ?", data.ID).Delete(Message{}).Error
	return tmpl, err
}

func HandleLogsHTML(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	idString := pat.Param(ctx, "id")

	parsed, err := strconv.ParseInt(idString, 10, 64)
	if web.CheckErr(tmpl, err, "Thats's not a real log id", nil) {
		return tmpl
	}

	msgLogs, err := GetChannelLogs(parsed)
	if web.CheckErr(tmpl, err, "Failed retrieving message logs", logrus.Error) {
		return tmpl
	}

	if msgLogs.GuildID != g.ID {
		return tmpl.AddAlerts(web.ErrorAlert("Couldn't find the logs im so sorry please dont hurt me i have a family D:"))
	}

	for k, v := range msgLogs.Messages {
		parsed, err := discordgo.Timestamp(v.Timestamp).Parse()
		if err != nil {
			logrus.WithError(err).Error("Failed parsing logged message timestamp")
			continue
		}
		ts := parsed.UTC().Format(time.RFC822)
		msgLogs.Messages[k].Timestamp = ts
	}

	tmpl["Logs"] = msgLogs
	return tmpl
}
