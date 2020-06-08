package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/models"
	"github.com/jonas747/yagpdb/common/patreon"
	"github.com/jonas747/yagpdb/web/discordblog"
	"github.com/mediocregopher/radix/v3"
	"github.com/patrickmn/go-cache"
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
	common.RedisPool.Do(radix.Cmd(&joinedServers, "SCARD", "connected_guilds"))

	tmpl["JoinedServers"] = joinedServers
	tmpl["DemoServerID"] = confDemoServerID.GetString()

	// Command stats
	tmpl["Commands"] = atomic.LoadInt64(commandsRanToday)

	return tmpl, nil
}

// BotStatus represents the bot's full status
type BotStatus struct {
	// Invidual statuses
	HostStatuses []*HostStatus `json:"host_statuses"`
	NumNodes     int           `json:"num_nodes"`
	TotalShards  int           `json:"total_shards"`

	UnavailableGuilds int   `json:"unavailable_guilds"`
	OfflineShards     []int `json:"offline_shards"`

	EventsPerSecondAverage float64 `json:"events_per_second_average"`
	EventsPerSecondMin     float64 `json:"events_per_second_min"`
	EventsPerSecondMax     float64 `json:"events_per_second_max"`

	UptimeMax time.Duration `json:"uptim_emax"`
	UptimeMin time.Duration `json:"uptime_min"`
}

var (
	// cached the status for a couple seconds to prevent DOS vector since this is a heavy endpoint
	cachedBotStatus *BotStatus
	botStatusCacheT time.Time
	botStatusCacheL sync.Mutex
)

func getFullBotStatus() (*BotStatus, error) {
	botStatusCacheL.Lock()
	defer botStatusCacheL.Unlock()
	if time.Since(botStatusCacheT) < time.Second*5 {
		return cachedBotStatus, nil
	}

	fullStatus, err := botrest.GetNodeStatuses()
	if err != nil {
		return nil, err
	}

	status := &BotStatus{
		NumNodes:    len(fullStatus.Nodes),
		TotalShards: fullStatus.TotalShards,

		EventsPerSecondMin: -1,
		EventsPerSecondMax: -1,

		UptimeMax: -1,
		UptimeMin: -1,

		OfflineShards: fullStatus.MissingShards,
	}

	totalEventsPerSecond := float64(-1)

	sort.Slice(fullStatus.Nodes, func(i, j int) bool {
		return fullStatus.Nodes[i].Uptime > fullStatus.Nodes[j].Uptime
	})

	// group by hosts and calculate stats
	for _, node := range fullStatus.Nodes {
		var host *HostStatus

		for _, v := range status.HostStatuses {
			if v.Name == node.Host {
				host = v
				break
			}
		}

		if host == nil {
			host = &HostStatus{
				Name: node.Host,
			}
			status.HostStatuses = append(status.HostStatuses, host)
		}

		host.Nodes = append(host.Nodes, node)

		// update stats
		for _, v := range node.Shards {
			if v.EventsPerSecond < status.EventsPerSecondMin || status.EventsPerSecondMin == -1 {
				status.EventsPerSecondMin = v.EventsPerSecond
			}

			if v.EventsPerSecond > status.EventsPerSecondMax || status.EventsPerSecondMax == -1 {
				status.EventsPerSecondMax = v.EventsPerSecond
			}

			totalEventsPerSecond += v.EventsPerSecond

			if v.ConnStatus != discordgo.GatewayStatusReady {
				status.OfflineShards = append(status.OfflineShards, v.ShardID)
			}

			status.UnavailableGuilds += v.UnavailableGuilds
		}

		if status.UptimeMin == -1 || node.Uptime < status.UptimeMin {
			status.UptimeMin = node.Uptime
		}
		if status.UptimeMax == -1 || node.Uptime > status.UptimeMax {
			status.UptimeMax = node.Uptime
		}
	}

	status.EventsPerSecondAverage = totalEventsPerSecond / float64(fullStatus.TotalShards)

	cachedBotStatus = status
	botStatusCacheT = time.Now()

	return status, nil
}

// HandleStatusHTML handles GET /status
func HandleStatusHTML(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	_, tmpl := GetCreateTemplateData(r.Context())

	status, err := getFullBotStatus()
	if err != nil {
		return tmpl, err
	}

	tmpl["BotStatus"] = status

	return tmpl, nil
}

// HandleStatusJSON handles GET /status.json
func HandleStatusJSON(w http.ResponseWriter, r *http.Request) interface{} {
	status, err := getFullBotStatus()
	if err != nil {
		return err
	}

	return status
}

type HostStatus struct {
	Name string

	EventsPerSecond float64
	TotalEvents     int64

	Nodes []*botrest.NodeStatus
}

func genFakeNodeStatuses(hosts int, nodes int, shards int) []*HostStatus {
	result := make([]*HostStatus, 0, hosts)

	for hostI := 0; hostI < hosts; hostI++ {
		host := &HostStatus{
			Name: "yagpdb-" + strconv.Itoa(hostI),
		}
		for nodeI := 0; nodeI < nodes; nodeI++ {

			nodeID := (hostI * nodes) + nodeI

			ns := &botrest.NodeStatus{
				ID: strconv.Itoa(nodeID),
			}

			for shardI := 0; shardI < shards; shardI++ {
				shard := &botrest.ShardStatus{
					ShardID:         (nodeID * shards) + shardI,
					TotalEvents:     999999,
					EventsPerSecond: rand.Float64() * 100,

					ConnStatus: discordgo.GatewayStatus(rand.Intn(5)),

					LastHeartbeatSend: time.Now().Add(-time.Second * 10),
					LastHeartbeatAck:  time.Now(),
				}
				ns.Shards = append(ns.Shards, shard)

				host.EventsPerSecond += shard.EventsPerSecond
				host.TotalEvents += shard.TotalEvents
			}

			host.Nodes = append(host.Nodes, ns)
		}

		result = append(result, host)
	}

	return result
}

func HandleReconnectShard(w http.ResponseWriter, r *http.Request) (TemplateData, error) {
	ctx, tmpl := GetCreateTemplateData(r.Context())
	tmpl["VisibleURL"] = "/status"

	if user := ctx.Value(common.ContextKeyUser); user != nil {
		cast := user.(*discordgo.User)
		if !common.IsOwner(cast.ID) {
			return HandleStatusHTML(w, r)
		}
	} else {
		return HandleStatusHTML(w, r)
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
	return HandleStatusHTML(w, r)
}

func HandleChanenlPermissions(w http.ResponseWriter, r *http.Request) interface{} {
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

func handleAdsTXT(w http.ResponseWriter, r *http.Request) {
	adsPath := ConfAdsTxt.GetString()
	if adsPath == "" {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(``))
		return
	}

	f, err := ioutil.ReadFile(ConfAdsTxt.GetString())
	if err != nil {
		logger.WithError(err).Error("failed reading ads.txt file")
		return
	}

	w.Write(f)
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
	<li>Read-only roles: <code>%d</code></li>
	<li>Write roles: <code>%d</code></li>
	<li>All members read-only: %s</li>
	<li>Allow absolutely everyone read-only access: %s</li>
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

	nilled := make([]*common.GuildWithConnected, len(wrapped))
	var wg sync.WaitGroup
	wg.Add(len(wrapped))

	// the servers the user is on and the user has manage server perms
	for i, g := range wrapped {
		go func(j int, gwc *common.GuildWithConnected) {
			conf := common.GetCoreServerConfCached(gwc.ID)
			if HasAccesstoGuildSettings(user.ID, gwc, conf, basicRoleProvider, false) {
				nilled[j] = gwc
			}

			wg.Done()
		}(i, g)
	}

	wg.Wait()

	accessibleGuilds := make([]*common.GuildWithConnected, 0, len(wrapped))
	for _, v := range nilled {
		if v != nil {
			accessibleGuilds = append(accessibleGuilds, v)
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
