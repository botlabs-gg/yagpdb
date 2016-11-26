package logs

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

func (lp *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/public_channel_logs.html"))

	web.ServerPublicMux.HandleC(pat.Get("/logs/:id"), web.RenderHandler(HandleLogsHTML, "public_server_logs"))
	web.ServerPublicMux.HandleC(pat.Get("/logs/:id/"), web.RenderHandler(HandleLogsHTML, "public_server_logs"))
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
