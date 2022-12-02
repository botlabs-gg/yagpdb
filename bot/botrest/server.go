package botrest

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"goji.io"
	"goji.io/pat"
)

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

var (
	// _ bot.BotInitHandler = (*Plugin)(nil)
	_ internalapi.InternalAPIPlugin = (*Plugin)(nil)
)

var serverLogger = common.GetFixedPrefixLogger("botrest_server")

// type BotRestPlugin interface {
// 	InitBotRestServer(mux *goji.Mux)
// }

type Plugin struct {
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "BotREST",
		SysName:  "botrest",
		Category: common.PluginCategoryCore,
	}
}

func (p *Plugin) InitInternalAPIRoutes(muxer *goji.Mux) {
	if !bot.Enabled {
		// bot only routes
		return
	}

	// muxer := goji.NewMux()

	muxer.HandleFunc(pat.Get("/:guild/guild"), HandleGuild)
	muxer.HandleFunc(pat.Get("/:guild/botmember"), HandleBotMember)
	muxer.HandleFunc(pat.Get("/:guild/members"), HandleGetMembers)
	muxer.HandleFunc(pat.Get("/:guild/membercolors"), HandleGetMemberColors)
	muxer.HandleFunc(pat.Get("/:guild/onlinecount"), HandleGetOnlineCount)
	muxer.HandleFunc(pat.Get("/:guild/channelperms/:channel"), HandleChannelPermissions)
	muxer.HandleFunc(pat.Get("/node_status"), HandleNodeStatus)
	muxer.HandleFunc(pat.Get("/shard_sessions"), HandleGetShardSessions)
	muxer.HandleFunc(pat.Post("/shard/:shard/reconnect"), HandleReconnectShard)

	// for _, p := range common.Plugins {
	// 	if botRestPlugin, ok := p.(BotRestPlugin); ok {
	// 		botRestPlugin.InitBotRestServer(muxer)
	// 	}
	// }

}

func HandleGuild(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)

	guild := bot.State.GetGuild(gId)
	if guild == nil {
		internalapi.ServerError(w, r, errors.New("Guild not found"))
		return
	}

	internalapi.ServeJson(w, r, guild)
}

func HandleBotMember(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)

	member, err := bot.GetMember(gId, common.BotUser.ID)
	if err != nil {
		internalapi.ServerError(w, r, errors.New("Bot Member not found"))
		return
	}

	internalapi.ServeJson(w, r, member.DgoMember())
}

func HandleGetMembers(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	uIDs, ok := r.URL.Query()["users"]

	if !ok || len(uIDs) < 1 {
		internalapi.ServerError(w, r, errors.New("No id's provided"))
		return
	}

	if len(uIDs) > 100 {
		internalapi.ServerError(w, r, errors.New("Too many ids provided"))
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
		members[i] = v.DgoMember()
	}

	internalapi.ServeJson(w, r, members)
}

func HandleGetMemberColors(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	uIDs, ok := r.URL.Query()["users"]

	if !ok || len(uIDs) < 1 {
		internalapi.ServerError(w, r, errors.New("No id's provided"))
		return
	}

	if len(uIDs) > 100 {
		internalapi.ServerError(w, r, errors.New("Too many ids provided"))
		return
	}

	guild := bot.State.GetGuild(gId)
	if guild == nil {
		internalapi.ServerError(w, r, errors.New("Guild not found"))
		return
	}

	uIDsParsed := make([]int64, 0, len(uIDs))
	for _, v := range uIDs {
		parsed, _ := strconv.ParseInt(v, 10, 64)
		uIDsParsed = append(uIDsParsed, parsed)
	}

	memberStates, _ := bot.GetMembers(gId, uIDsParsed...)

	colors := make(map[string]int)
	for _, ms := range memberStates {
		// Find the highest role this user has with a color
		for _, role := range guild.Roles {
			if role.Color == 0 {
				continue
			}

			if !common.ContainsInt64Slice(ms.Member.Roles, role.ID) {
				continue
			}

			// Bingo
			colors[strconv.FormatInt(ms.User.ID, 10)] = role.Color
			break
		}
	}

	internalapi.ServeJson(w, r, colors)
}

func HandleGetOnlineCount(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)

	count := 0

	bot.State.IterateMembers(gId, func(chunk []*dstate.MemberState) bool {
		count += len(chunk)
		return true
	})

	internalapi.ServeJson(w, r, count)
}

func HandleChannelPermissions(w http.ResponseWriter, r *http.Request) {
	gId, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	cId, _ := strconv.ParseInt(pat.Param(r, "channel"), 10, 64)

	guild := bot.State.GetGuild(gId)
	if guild == nil {
		internalapi.ServerError(w, r, errors.New("Guild not found"))
		return
	}

	member, err := bot.GetMember(gId, common.BotUser.ID)
	if err != nil {
		internalapi.ServerError(w, r, errors.New("Could not find bot member"))
		return
	}

	perms, err := guild.GetMemberPermissions(cId, member.User.ID, member.Member.Roles)
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "Error calculating perms"))
		return
	}

	internalapi.ServeJson(w, r, perms)
}

func HandlePing(w http.ResponseWriter, r *http.Request) {
	internalapi.ServeJson(w, r, "pong")
}

type ShardStatus struct {
	ShardID         int     `json:"shard_id"`
	TotalEvents     int64   `json:"total_events"`
	EventsPerSecond float64 `json:"events_per_second"`

	ConnStatus discordgo.GatewayStatus `json:"conn_status"`

	LastHeartbeatSend time.Time `json:"last_heartbeat_send"`
	LastHeartbeatAck  time.Time `json:"last_heartbeat_ack"`

	NumGuilds         int
	UnavailableGuilds int
}

func HandleNodeStatus(w http.ResponseWriter, r *http.Request) {

	totalEventStats, periodEventStats := bot.EventLogger.GetStats()

	numShards := bot.ShardManager.GetNumShards()
	result := make([]*ShardStatus, 0, numShards)

	processShards := bot.ReadyTracker.GetProcessShards()

	// get general shard stats
	for _, shardID := range processShards {
		shard := bot.ShardManager.Sessions[shardID]

		sumEvents := int64(0)
		sumPeriodEvents := int64(0)

		for j := range totalEventStats[shardID] {
			sumEvents += totalEventStats[shardID][j]
			sumPeriodEvents += periodEventStats[shardID][j]
		}

		if shard == nil || shard.GatewayManager == nil {
			result[shardID] = &ShardStatus{ConnStatus: discordgo.GatewayStatusDisconnected}
			continue
		}

		beat, ack := shard.GatewayManager.HeartBeatStats()

		guilds := bot.State.GetShardGuilds(int64(shardID))
		numUnavailable := 0
		for _, g := range guilds {
			if !g.Available {
				numUnavailable++
			}
		}

		result = append(result, &ShardStatus{
			ShardID:           shardID,
			ConnStatus:        shard.GatewayManager.Status(),
			TotalEvents:       sumEvents,
			EventsPerSecond:   float64(sumPeriodEvents) / bot.EventLoggerPeriodDuration.Seconds(),
			LastHeartbeatSend: beat,
			LastHeartbeatAck:  ack,
			NumGuilds:         len(guilds),
			UnavailableGuilds: numUnavailable,
		})
	}

	hostname, _ := os.Hostname()

	internalapi.ServeJson(w, r, &NodeStatus{
		Host:   hostname,
		Shards: result,
		ID:     common.NodeID,
		Uptime: time.Since(bot.Started),
	})
}

type shardSessionInfo struct {
	ShardID          int
	SessionID        string
	ResumeGatewayUrl string
}

func HandleGetShardSessions(w http.ResponseWriter, r *http.Request) {

	// numShards := bot.ShardManager.GetNumShards()
	// result := make([]*ShardStatus, 0, numShards)

	processShards := bot.ReadyTracker.GetProcessShards()

	result := make([]*shardSessionInfo, 0)

	// get general shard stats
	for _, shardID := range processShards {
		shard := bot.ShardManager.Sessions[shardID]
		sessionID, _, resumeGatewayUrl := shard.GatewayManager.GetSessionInfo()
		result = append(result, &shardSessionInfo{
			ShardID:          shardID,
			SessionID:        sessionID,
			ResumeGatewayUrl: resumeGatewayUrl,
		})
	}

	internalapi.ServeJson(w, r, result)
}

func HandleReconnectShard(w http.ResponseWriter, r *http.Request) {
	sID := pat.Param(r, "shard")
	forceReidentify := r.FormValue("reidentify") == "1"
	if sID == "*" {
		go RestartAll(forceReidentify)
		internalapi.ServeJson(w, r, "ok")
		return
	}

	parsed, _ := strconv.ParseInt(sID, 10, 32)
	shardcount := bot.ShardManager.GetNumShards()
	if parsed < 0 || int(parsed) >= shardcount {
		internalapi.ServerError(w, r, errors.New("Unknown shard"))
		return
	}

	if forceReidentify {
		err := bot.ShardManager.Sessions[parsed].Close()
		if err != nil {
			internalapi.ServerError(w, r, errors.New("failed stopping shard: "+err.Error()))
			return
		}
	}

	sessionID, sequence, resumeGatewayUrl := bot.ShardManager.Sessions[parsed].GatewayManager.GetSessionInfo()

	var err error
	if forceReidentify {
		bot.ShardManager.Sessions[parsed].GatewayManager.SetSessionInfo("", 0, "")
		err = bot.ShardManager.Sessions[parsed].GatewayManager.Open()
	} else {
		err = bot.ShardManager.Sessions[parsed].GatewayManager.Reconnect(forceReidentify)
	}
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "Reconnect"))
		return
	}

	internalapi.ServeJson(w, r, ReconnectResponse{
		SessionID:        sessionID,
		Sequence:         sequence,
		ResumeGatewayUrl: resumeGatewayUrl,
	})
}

type ReconnectResponse struct {
	SessionID        string `json:"session_id"`
	Sequence         int64  `json:"sequence"`
	ResumeGatewayUrl string `json:"resume_gateway_url"`
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
