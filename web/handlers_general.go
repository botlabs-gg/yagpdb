package web

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"net/http"
)

func HandleCPLogs(w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := GetBaseCPContextData(r.Context())

	logs, err := common.GetCPLogEntries(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(ErrorAlert("Failed retrieving logs", err))
	} else {
		templateData["entries"] = logs
	}
	return templateData
}

func HandleSelectServer(w http.ResponseWriter, r *http.Request) interface{} {
	_, tmpl := GetCreateTemplateData(r.Context())

	if r.FormValue("guild_id") != "" {
		guild, err := common.BotSession.Guild(r.FormValue("guild_id"))
		if err != nil {
			logrus.WithError(err).WithField("guild", r.FormValue("guild_id")).Error("Failed fetching guild")
			return tmpl
		}

		tmpl["JoinedGuild"] = guild
	}

	// g, _ := common.BotSession.Guild("140847179043569664")
	// tmpl["JoinedGuild"] = g

	return tmpl
}
