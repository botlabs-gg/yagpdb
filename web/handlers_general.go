package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/models"
	"github.com/jonas747/yagpdb/common/patreon"
	"github.com/jonas747/yagpdb/web/discordblog"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"goji.io/pat"
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
			CtxLogger(r.Context()).WithError(err).WithField("guild", r.FormValue("guild_id")).Error("Failed fetching guild")
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
	common.RedisPool.Do(retryableredis.Cmd(&joinedServers, "SCARD", "connected_guilds"))

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
		if cast.ID != int64(common.ConfOwner.GetInt()) {
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
			logger.WithError(err).Error("failed counting commands ran today")
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

type ControlPanelPlugin struct{}

func (p *ControlPanelPlugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Control Panel",
		SysName:  "control_panel",
		Category: common.PluginCategoryCore,
	}
}

var _ PluginWithServerHomeWidget = (*ControlPanelPlugin)(nil)

func (p *ControlPanelPlugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, templateData := GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Control Panel"
	templateData["SettingsPath"] = "/core"

	templateData["WidgetEnabled"] = true

	config := r.Context().Value(common.ContextKeyCoreConfig).(*models.CoreConfig)

	const format = `<ul>
	<li>Read Only roles: <code>%d</code></li>
	<li>Write Roles: <code>%d</code></li>
	<li>All members read only: %s</li>
	<li>Allow absolutely everyone read only access: %s</li>
</ul>`
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, len(config.AllowedReadOnlyRoles), len(config.AllowedWriteRoles), EnabledDisabledSpanStatus(config.AllowAllMembersReadOnly), EnabledDisabledSpanStatus(config.AllowNonMembersReadOnly)))

	return templateData, nil
}

func (p *ControlPanelPlugin) ServerHomeWidgetOrder() int {
	return 5
}

type CoreConfigPostForm struct {
	AllowedReadOnlyRoles    []int64 `valid:"role,true"`
	AllowedWriteRoles       []int64 `valid:"role,true"`
	AllowAllMembersReadOnly bool
	AllowNonMembersReadOnly bool
}

func HandlePostCoreSettings(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	g, templateData := GetBaseCPContextData(r.Context())

	form := r.Context().Value(common.ContextKeyParsedForm).(*CoreConfigPostForm)

	m := &models.CoreConfig{
		GuildID:              g.ID,
		AllowedReadOnlyRoles: form.AllowedReadOnlyRoles,
		AllowedWriteRoles:    form.AllowedWriteRoles,

		AllowAllMembersReadOnly: form.AllowAllMembersReadOnly,
		AllowNonMembersReadOnly: form.AllowNonMembersReadOnly,
	}

	err := common.CoreConfigSave(r.Context(), m)
	if err != nil {
		return templateData, err
	}

	templateData["CoreConfig"] = m

	return templateData, nil
}

func HandleGetManagedGuilds(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	ctx := r.Context()
	_, templateData := GetBaseCPContextData(ctx)

	user := ContextUser(ctx)

	// retrieve guilds this user is part of
	// i really wish there was a easy to to invalidate this cache, but since there's not it just expires after 10 seconds
	wrapped, err := GetUserGuilds(ctx)
	if err != nil {
		return templateData, err
	}

	accessibleGuilds := make([]*common.GuildWithConnected, 0, len(wrapped))

	// the servers the user is on and the user has manage server perms
	for _, g := range wrapped {
		conf := common.GetCoreServerConfCached(g.ID)
		if HasAccesstoGuildSettings(user.ID, g, conf, basicRoleProvider, false) {
			accessibleGuilds = append(accessibleGuilds, g)
		}
	}

	templateData["ManagedGuilds"] = accessibleGuilds

	return templateData, nil
}

func basicRoleProvider(guildID, userID int64) []int64 {
	members, err := botrest.GetMembers(guildID, userID)
	if err != nil || len(members) < 1 || members[0] == nil {

		// fallback to discord api
		m, err := common.BotSession.GuildMember(guildID, userID)
		if err != nil {
			return nil
		}

		return m.Roles
	}

	return members[0].Roles
}

func GetUserGuilds(ctx context.Context) ([]*common.GuildWithConnected, error) {
	session := DiscordSessionFromContext(ctx)
	user := ContextUser(ctx)

	// retrieve guilds this user is part of
	// i really wish there was a easy to to invalidate this cache, but since there's not it just expires after 10 seconds
	var guilds []*discordgo.UserGuild
	err := common.GetCacheDataJson(discordgo.StrID(user.ID)+":guilds", &guilds)
	if err != nil {
		guilds, err = session.UserGuilds(100, 0, 0)
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("Failed getting user guilds")
			return nil, err
		}

		LogIgnoreErr(common.SetCacheDataJson(discordgo.StrID(user.ID)+":guilds", 10, guilds))
	}

	// wrap the guilds with some more info, such as wether the bot is on the server
	wrapped, err := common.GetGuildsWithConnected(guilds)
	if err != nil {
		CtxLogger(ctx).WithError(err).Error("Failed wrapping guilds")
		return nil, err
	}

	return wrapped, nil
}

var WidgetCache = cache.New(time.Second*10, time.Second*10)

type WidgetCacheItem struct {
	RawResponse []byte
	Header      http.Header
}

// Writes the request log into logger, returns a new middleware
func GuildScopeCacheMW(plugin common.Plugin, inner http.Handler) http.Handler {

	mw := func(w http.ResponseWriter, r *http.Request) {
		g, _ := GetBaseCPContextData(r.Context())

		cacheKey := discordgo.StrID(g.ID) + "_" + plugin.PluginInfo().SysName

		if v, ok := WidgetCache.Get(cacheKey); ok {
			// already in the cache
			cast := v.(*WidgetCacheItem)
			for headerKey, headerValue := range cast.Header {
				w.Header()[headerKey] = headerValue
			}
			w.WriteHeader(200)
			w.Write(cast.RawResponse)
			// CtxLogger(r.Context()).Info("cache hit")
			return
		}

		// CtxLogger(r.Context()).Info("cache miss")

		// create the multiwrite and put it in the cache
		var b bytes.Buffer
		newW := io.MultiWriter(w, &b)

		newRW := &CustomResponseWriter{
			inner: w,
			newW:  newW,
		}

		inner.ServeHTTP(newRW, r)

		item := &WidgetCacheItem{
			RawResponse: b.Bytes(),
			Header:      w.Header(),
		}
		WidgetCache.Set(cacheKey, item, time.Second*10)
	}

	return http.HandlerFunc(mw)
}

type CustomResponseWriter struct {
	inner http.ResponseWriter
	newW  io.Writer
}

func (c *CustomResponseWriter) Header() http.Header {
	return c.inner.Header()
}

func (c *CustomResponseWriter) Write(b []byte) (int, error) {
	return c.newW.Write(b)
}

func (c *CustomResponseWriter) WriteHeader(statusCode int) {
	c.inner.WriteHeader(statusCode)
}
