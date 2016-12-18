package web

import (
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
