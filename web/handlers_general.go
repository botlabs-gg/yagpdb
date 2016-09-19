package web

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/nhooyr/color/log"
	"golang.org/x/net/context"
	"net/http"
)

func IndexHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {

	templateData := TemplateData(make(map[string]interface{}))

	if val := ctx.Value(ContextKeyTemplateData); val != nil {
		templateData = val.(TemplateData)
	}
	return templateData
}

func HandleSelectServer(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(ContextKeyTemplateData).(TemplateData)

	err := Templates.ExecuteTemplate(w, "cp_selectserver", templateData)
	if err != nil {
		log.Println("Failed executing templae", err)
	}
}

func HandleCPLogs(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := GetBaseCPContextData(ctx)
	templateData["current_page"] = "cp_logs"

	logs, err := common.GetCPLogEntries(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(ErrorAlert("Failed retrieving logs", err))
	} else {
		templateData["entries"] = logs
	}
	LogIgnoreErr(Templates.ExecuteTemplate(w, "cp_action_logs", templateData))
}
