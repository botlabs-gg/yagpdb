package web

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"net/http"
	"time"
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

func HandleLandingPage(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, tmpl := GetCreateTemplateData(r.Context())
	redis := RedisClientFromContext(r.Context())

	joinedServers, _ := redis.Cmd("SCARD", "connected_guilds").Int()

	tmpl["JoinedServers"] = joinedServers

	// Command stats
	within := time.Now().Add(-24 * time.Hour)

	var result struct {
		Count int64
	}
	err := common.GORM.Table(common.LoggedExecutedCommand{}.TableName()).Select("COUNT(*)").Where("created_at > ?", within).Scan(&result).Error
	if err != nil {
		return tmpl, err
	}

	tmpl["Commands"] = result.Count

	return tmpl, nil
}
