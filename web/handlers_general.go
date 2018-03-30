package web

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web/blog"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"goji.io/pat"
	"net/http"
	"strconv"
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

	joinedGuildParsed, _ := strconv.ParseInt(r.FormValue("guild_id"), 10, 64)
	if joinedGuildParsed != 0 {
		guild, err := common.BotSession.Guild(joinedGuildParsed)
		if err != nil {
			logrus.WithError(err).WithField("guild", r.FormValue("guild_id")).Error("Failed fetching guild")
		} else {
			tmpl["JoinedGuild"] = guild
		}
	}

	offset := 0
	if r.FormValue("offset") != "" {
		offset, _ = strconv.Atoi(r.FormValue("offset"))
	}

	if r.FormValue("post_id") != "" {
		id, _ := strconv.Atoi(r.FormValue("post_id"))
		p := blog.GetPost(id)
		if p != nil {
			tmpl["Posts"] = []*blog.Post{p}
		} else {
			tmpl.AddAlerts(ErrorAlert("Post not found"))
		}
	} else {
		posts := blog.GetPostsNewest(5, offset)
		tmpl["Posts"] = posts
		if len(posts) > 4 {
			tmpl["NextPostsOffset"] = offset + 5
		}
		if offset != 0 {
			tmpl["CurrentPostsOffset"] = offset
			previous := offset - 5
			if previous < 0 {
				previous = 0
			}
			tmpl["PreviousPostsOffset"] = previous
		}
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

	if user := ctx.Value(common.ContextKeyUser); user != nil {
		cast := user.(*discordgo.User)
		if cast.ID != common.Conf.Owner {
			return HandleStatus(w, r)
		}
	} else {
		return HandleStatus(w, r)
	}

	CtxLogger(ctx).Info("Triggering reconnect...")

	sID := pat.Param(r, "shard")
	parsed, _ := strconv.ParseInt(sID, 10, 32)

	err := botrest.SendReconnectShard(int(parsed))
	if err != nil {
		return tmpl, err
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
