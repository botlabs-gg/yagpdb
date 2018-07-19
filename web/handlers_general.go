package web

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web/discordblog"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"goji.io/pat"
	"net/http"
	"strconv"
	"time"
)

func HandleCPLogs(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, templateData := GetBaseCPContextData(r.Context())

	logs, err := common.GetCPLogEntries(activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(ErrorAlert("Failed retrieving logs", err))
	} else {
		templateData["entries"] = logs
	}
	return templateData
}

func HandleSelectServer(w http.ResponseWriter, r *http.Request) interface{} {
	_, tmpl := GetCreateTemplateData(r.Context())

	joinedGuildParsed, _ := strconv.ParseInt(r.FormValue("guild_id"), 10, 64)
	if joinedGuildParsed != 0 {
		guild, err := common.BotSession.Guild(joinedGuildParsed)
		if err != nil {
			logrus.WithError(err).WithField("guild", r.FormValue("guild_id")).Error("Failed fetching guild")
		} else {
			tmpl["JoinedGuild"] = guild
		}
	}

	posts := discordblog.GetNewestPosts(10)
	tmpl["Posts"] = posts

	return tmpl
}

func HandleLandingPage(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, tmpl := GetCreateTemplateData(r.Context())

	var joinedServers int
	common.RedisPool.Do(radix.Cmd(&joinedServers, "SCARD", "connected_guilds"))

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

func HandleStatus(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, tmpl := GetCreateTemplateData(r.Context())

	statuses, err := botrest.GetShardStatuses()
	if err != nil {
		return tmpl, err
	}

	tmpl["Shards"] = statuses

	return tmpl, nil
}

func HandleReconnectShard(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	ctx, tmpl := GetCreateTemplateData(r.Context())
	tmpl["VisibleURL"] = "/status"

	if user := ctx.Value(common.ContextKeyUser); user != nil {
		cast := user.(*discordgo.User)
		if cast.ID != common.Conf.Owner {
			return HandleStatus(w, r)
		}
	} else {
		return HandleStatus(w, r)
	}

	CtxLogger(ctx).Info("Triggering reconnect...", r.FormValue("reidentify"))
	identify := r.FormValue("reidentify") == "1"

	var err error
	sID := pat.Param(r, "shard")
	if sID != "*" {
		parsed, _ := strconv.ParseInt(sID, 10, 32)
		err = botrest.SendReconnectShard(int(parsed), identify)
	} else {
		err = botrest.SendReconnectAll(identify)
	}

	if err != nil {
		tmpl.AddAlerts(ErrorAlert(err.Error()))
	}
	return HandleStatus(w, r)
}

func HandleChanenlPermissions(w http.ResponseWriter, r *http.Request) interface{} {
	if !botrest.BotIsRunning() {
		return errors.New("Bot is not responding")
	}

	g := r.Context().Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)
	c, _ := strconv.ParseInt(pat.Param(r, "channel"), 10, 64)
	perms, err := botrest.GetChannelPermissions(g.ID, c)
	if err != nil {
		return err
	}

	return perms
}
