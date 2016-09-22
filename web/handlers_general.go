package web

import (
	"github.com/jonas747/yagpdb/common"
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

func HandleCPLogs(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := GetBaseCPContextData(ctx)

	logs, err := common.GetCPLogEntries(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(ErrorAlert("Failed retrieving logs", err))
	} else {
		templateData["entries"] = logs
	}
	return templateData
}
