package web

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/patreon"
	"github.com/jonas747/yagpdb/web/discordblog"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"goji.io/pat"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

type serverHomeWidget struct {
	HumanName  string
	PluginName string
	Plugin     common.Plugin
}

type serverHomeWidgetCategory struct {
	Category *common.PluginCategory
	Widgets  []*serverHomeWidget
}

func HandleServerHome(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, templateData := GetBaseCPContextData(r.Context())

	containers := make([]*serverHomeWidgetCategory, 0)

	for _, v := range common.Plugins {
		if _, ok := v.(PluginWithServerHomeWidget); ok {

			// find the category
			var cat *serverHomeWidgetCategory
			for _, c := range containers {
				if c.Category == v.PluginInfo().Category {
					cat = c
					break
				}
			}

			if cat == nil {
				// meow
				cat = &serverHomeWidgetCategory{
					Category: v.PluginInfo().Category,
				}
				containers = append(containers, cat)
			}

			cat.Widgets = append(cat.Widgets, &serverHomeWidget{
				HumanName:  v.PluginInfo().Name,
				PluginName: v.PluginInfo().SysName,
				Plugin:     v,
			})
		}
	}

	sort.Slice(containers, func(i, j int) bool {
		return containers[i].Category.Order < containers[j].Category.Order
	})

	// order the widgets within the containers
	for _, c := range containers {
		sort.Slice(c.Widgets, func(i, j int) bool {
			iOrder := 1000000
			jOrder := 1000000

			if cast, ok := c.Widgets[i].Plugin.(ServerHomeWidgetWithOrder); ok {
				iOrder = cast.ServerHomeWidgetOrder()
			}

			if cast, ok := c.Widgets[j].Plugin.(ServerHomeWidgetWithOrder); ok {
				jOrder = cast.ServerHomeWidgetOrder()
			}

			return iOrder < jOrder
		})
	}

	templateData["PluginContainers"] = containers

	return templateData, nil
}

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

	if patreon.ActivePoller != nil {
		patrons := patreon.ActivePoller.GetPatrons()
		if len(patrons) > 0 {
			tmpl["patreonActive"] = true
			tmpl["activePatrons"] = patrons
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
	tmpl["Commands"] = atomic.LoadInt64(commandsRanToday)

	return tmpl, nil
}

func HandleStatus(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, tmpl := GetCreateTemplateData(r.Context())

	nodes, err := botrest.GetNodeStatuses()
	if err != nil {
		return tmpl, err
	}

	tmpl["Nodes"] = nodes

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

var commandsRanToday = new(int64)

func pollCommandsRan() {
	t := time.NewTicker(time.Minute)
	for {
		var result struct {
			Count int64
		}

		within := time.Now().Add(-24 * time.Hour)

		err := common.GORM.Table(common.LoggedExecutedCommand{}.TableName()).Select("COUNT(*)").Where("created_at > ?", within).Scan(&result).Error
		if err != nil {
			logrus.WithError(err).Error("failed counting commands ran today")
		} else {
			atomic.StoreInt64(commandsRanToday, result.Count)
		}

		<-t.C
	}
}

func handleRobotsTXT(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`User-agent: *
Disallow: /manage/
`))
}
