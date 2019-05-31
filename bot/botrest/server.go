package botrest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/dutil"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/pkg/errors"
	"goji.io"
	"goji.io/pat"
)

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

var (
	_ bot.BotInitHandler    = (*Plugin)(nil)
	_ bot.BotStopperHandler = (*Plugin)(nil)
)

var confBotrestListenAddr = config.RegisterOption("yagpdb.botrest.listen_address", "botrest listening address, it will use the first port available above 5010", "127.0.0.1")
var serverLogger = common.GetFixedPrefixLogger("botrest_server")

type BotRestPlugin interface {
	InitBotRestServer(mux *goji.Mux)
}

type Plugin struct {
	srv   *http.Server
	srvMU sync.Mutex
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "BotREST",
		SysName:  "botrest",
		Category: common.PluginCategoryCore,
	}
}

func (p *Plugin) BotInit() {

	muxer := goji.NewMux()

	muxer.HandleFunc(pat.Get("/:guild/guild"), HandleGuild)
	muxer.HandleFunc(pat.Get("/:guild/botmember"), HandleBotMember)
	muxer.HandleFunc(pat.Get("/:guild/members"), HandleGetMembers)
	muxer.HandleFunc(pat.Get("/:guild/membercolors"), HandleGetMemberColors)
	muxer.HandleFunc(pat.Get("/:guild/onlinecount"), HandleGetOnlineCount)
	muxer.HandleFunc(pat.Get("/:guild/channelperms/:channel"), HandleChannelPermissions)
	muxer.HandleFunc(pat.Get("/gw_status"), HandleGWStatus)
	muxer.HandleFunc(pat.Post("/shard/:shard/reconnect"), HandleReconnectShard)
	muxer.HandleFunc(pat.Get("/ping"), HandlePing)

	// Debug stuff
	muxer.HandleFunc(pat.Get("/debug/pprof/*"), pprof.Index)
	muxer.HandleFunc(pat.Get("/debug/pprof"), pprof.Index)
	muxer.HandleFunc(pat.Get("/debug2/pproff/cmdline"), pprof.Cmdline)
	muxer.HandleFunc(pat.Get("/debug2/pproff/profile"), pprof.Profile)
	muxer.HandleFunc(pat.Get("/debug2/pproff/symbol"), pprof.Symbol)
	muxer.HandleFunc(pat.Get("/debug2/pproff/trace"), pprof.Trace)

	for _, p := range common.Plugins {
		if botRestPlugin, ok := p.(BotRestPlugin); ok {
			botRestPlugin.InitBotRestServer(muxer)
		}
	}

	p.srv = &http.Server{
		Handler: muxer,
	}

	currentPort := 5010

	go func() {
		// listen address excluding port
		listenAddr := confBotrestListenAddr.GetString()
		if listenAddr == "" {
			// default to safe loopback interface
			listenAddr = "127.0.0.1"
		}

		for {
			address := listenAddr + ":" + strconv.Itoa(currentPort)

			serverLogger.Println("starting botrest on ", address)

			p.srvMU.Lock()
			p.srv.Addr = address
			p.srvMU.Unlock()

			err := p.srv.ListenAndServe()
			if err != nil {
				// Shutdown was called for graceful shutdown
				if err == http.ErrServerClosed {
					serverLogger.Info("server closed, shutting down...")
					return
				}

				// Retry with a higher port until we succeed
				serverLogger.WithError(err).Error("failed starting botrest http server on ", address, " trying again on next port")
				currentPort++
				time.Sleep(time.Millisecond)
				continue
			}

			serverLogger.Println("botrest returned without any error")
			break
		}
	}()

	// Wait for the server address to stop changing
	go func() {
		lastAddr := ""
		lastChange := time.Now()
		for {
			p.srvMU.Lock()
			addr := p.srv.Addr
			p.srvMU.Unlock()

			if lastAddr != addr {
				lastAddr = addr
				time.Sleep(time.Second)
				lastChange = time.Now()
				continue
			}

			if time.Since(lastChange) > time.Second*5 {
				// found avaiable port
				go p.mapper(lastAddr)
				return
			}

			time.Sleep(time.Second)
		}
	}()
}

func (p *Plugin) mapper(address string) {
	t := time.NewTicker(time.Second * 10)
	for {
		p.mapAddressToShards(address)
		<-t.C
	}
}

func (p *Plugin) mapAddressToShards(address string) {

	processShards := bot.GetProcessShards()

	// serverLogger.Debug("mapping ", address, " to current process shards")
	for _, shard := range processShards {
		err := common.RedisPool.Do(retryableredis.Cmd(nil, "SET", RedisKeyShardAddressMapping(shard), address))
		if err != nil {
			serverLogger.WithError(err).Error("failed mapping botrest")
		}
	}

	if bot.UsingOrchestrator {
		err := common.RedisPool.Do(retryableredis.Cmd(nil, "SET", RedisKeyNodeAddressMapping(bot.NodeConn.GetIDLock()), address))
		if err != nil {
			serverLogger.WithError(err).Error("failed mapping node")
		}
	}
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	p.srv.Shutdown(context.TODO())
	wg.Done()
}

func ServeJson(w http.ResponseWriter, r *http.Request, data interface{}) {
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		serverLogger.WithError(err).Error("Failed sending json")
	}
}

// Returns true if an error occured
func ServerError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	encodedErr, _ := json.Marshal(err.Error())

	w.WriteHeader(http.StatusInternalServerError)
	w.Write(encodedErr)
	return true
}

func HandleGuild(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	gCopy := guild.DeepCopy(true, true, false, true)

	ServeJson(w, r, gCopy)
}

func HandleBotMember(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	member := guild.MemberDGoCopy(true, common.BotUser.ID)
	if member == nil {
		ServerError(w, r, errors.New("Bot Member not found"))
		return
	}

	ServeJson(w, r, member)
}

func HandleGetMembers(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	uIDs, ok := r.URL.Query()["users"]

	if !ok || len(uIDs) < 1 {
		ServerError(w, r, errors.New("No id's provided"))
		return
	}

	if len(uIDs) > 100 {
		ServerError(w, r, errors.New("Too many ids provided"))
		return
	}

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	uIDsParsed := make([]int64, 0, len(uIDs))
	for _, v := range uIDs {
		parsed, _ := strconv.ParseInt(v, 10, 64)
		uIDsParsed = append(uIDsParsed, parsed)
	}

	memberStates, _ := bot.GetMembers(gId, uIDsParsed...)
	members := make([]*discordgo.Member, len(memberStates))
	for i, v := range memberStates {
		members[i] = v.DGoCopy()
	}

	ServeJson(w, r, members)
}

func HandleGetMemberColors(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	uIDs, ok := r.URL.Query()["users"]

	if !ok || len(uIDs) < 1 {
		ServerError(w, r, errors.New("No id's provided"))
		return
	}

	if len(uIDs) > 100 {
		ServerError(w, r, errors.New("Too many ids provided"))
		return
	}

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	uIDsParsed := make([]int64, 0, len(uIDs))
	for _, v := range uIDs {
		parsed, _ := strconv.ParseInt(v, 10, 64)
		uIDsParsed = append(uIDsParsed, parsed)
	}

	memberStates, _ := bot.GetMembers(gId, uIDsParsed...)

	guild.Lock()
	defer guild.Unlock()

	// Make sure the roles are in the proper order
	sort.Sort(dutil.Roles(guild.Guild.Roles))

	colors := make(map[string]int)
	for _, ms := range memberStates {
		// Find the highest role this user has with a color
		for _, role := range guild.Guild.Roles {
			if role.Color == 0 {
				continue
			}

			if !common.ContainsInt64Slice(ms.Roles, role.ID) {
				continue
			}

			// Bingo
			colors[ms.StrID()] = role.Color
			break
		}
	}

	ServeJson(w, r, colors)
}

func HandleGetOnlineCount(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	count := 0
	guild.RLock()
	for _, ms := range guild.Members {
		if ms.PresenceSet && ms.PresenceStatus != dstate.StatusNotSet && ms.PresenceStatus != dstate.StatusOffline {
			count++
		}
	}
	guild.RUnlock()

	ServeJson(w, r, count)
}

func HandleChannelPermissions(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	cId, _ := strconv.ParseInt(pat.Param(r, "channel"), 10, 64)

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	perms, err := guild.MemberPermissions(true, cId, common.BotUser.ID)

	if err != nil {
		ServerError(w, r, errors.WithMessage(err, "Error calculating perms"))
		return
	}

	ServeJson(w, r, perms)
}

func HandlePing(w http.ResponseWriter, r *http.Request) {
	ServeJson(w, r, "pong")
}

type ShardStatus struct {
	ShardID         int     `json:"shard_id"`
	TotalEvents     int64   `json:"total_events"`
	EventsPerSecond float64 `json:"events_per_second"`

	ConnStatus discordgo.GatewayStatus `json:"conn_status"`

	LastHeartbeatSend time.Time `json:"last_heartbeat_send"`
	LastHeartbeatAck  time.Time `json:"last_heartbeat_ack"`
}

func HandleGWStatus(w http.ResponseWriter, r *http.Request) {

	totalEventStats, periodEventStats := bot.EventLogger.GetStats()

	numShards := bot.ShardManager.GetNumShards()
	result := make([]*ShardStatus, 0, numShards)

	processShards := bot.GetProcessShards()

	for _, shardID := range processShards {
		shard := bot.ShardManager.Sessions[shardID]

		sumEvents := int64(0)
		sumPeriodEvents := int64(0)

		for j, _ := range totalEventStats[shardID] {
			sumEvents += totalEventStats[shardID][j]
			sumPeriodEvents += periodEventStats[shardID][j]
		}

		if shard == nil || shard.GatewayManager == nil {
			result[shardID] = &ShardStatus{ConnStatus: discordgo.GatewayStatusDisconnected}
			continue
		}

		beat, ack := shard.GatewayManager.HeartBeatStats()

		result = append(result, &ShardStatus{
			ShardID:           shardID,
			ConnStatus:        shard.GatewayManager.Status(),
			TotalEvents:       sumEvents,
			EventsPerSecond:   float64(sumPeriodEvents) / bot.EventLoggerPeriodDuration.Seconds(),
			LastHeartbeatSend: beat,
			LastHeartbeatAck:  ack,
		})
	}

	ServeJson(w, r, result)
}

func HandleReconnectShard(w http.ResponseWriter, r *http.Request) {
	sID := pat.Param(r, "shard")
	forceReidentify := r.FormValue("reidentify") == "1"
	if sID == "*" {
		go RestartAll(forceReidentify)
		ServeJson(w, r, "ok")
		return
	}

	parsed, _ := strconv.ParseInt(sID, 10, 32)
	shardcount := bot.ShardManager.GetNumShards()
	if parsed < 0 || int(parsed) >= shardcount {
		ServerError(w, r, errors.New("Unknown shard"))
		return
	}

	err := bot.ShardManager.Sessions[parsed].GatewayManager.Reconnect(forceReidentify)
	if err != nil {
		ServerError(w, r, errors.WithMessage(err, "Reconnect"))
		return
	}

	ServeJson(w, r, "ok")
}

func RestartAll(reidentify bool) {
	serverLogger.Println("Reconnecting all shards re-identify:", reidentify)
	for _, v := range bot.ShardManager.Sessions {
		err := v.GatewayManager.Reconnect(reidentify)
		if err != nil {
			serverLogger.WithError(err).Error("Failed reconnecting shard")
		}
		time.Sleep(time.Second * 5)
	}
}
