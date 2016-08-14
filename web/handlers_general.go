package web

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/nhooyr/color/log"
	"goji.io"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

func IndexHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {

	templateData := TemplateData(make(map[string]interface{}))

	if val := ctx.Value(ContextKeyTemplateData); val != nil {
		templateData = val.(TemplateData)
	}
	return templateData
}

// Will not serve pages unless a session is available
func RequestLoggerMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		inner.ServeHTTPC(ctx, w, r)
		duration := time.Since(started)

		durColor := "%h[fgGreen]"

		if duration.Seconds() > 0.25 {
			durColor = "%h[fgYellow]"
		}

		if duration.Seconds() > 1 {
			durColor = "%h[fgBrightRed]"
		}

		tsStr := ""
		if LogRequestTimestamps {
			tsStr = started.Format(time.Stamp) + ": "
		}

		address := r.RemoteAddr
		log.Printf(durColor+"%sHandled request [%4dms] %s: %s%r\n", tsStr, int(duration.Seconds()*1000), address, r.RequestURI)
	}
	return goji.HandlerFunc(mw)
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
