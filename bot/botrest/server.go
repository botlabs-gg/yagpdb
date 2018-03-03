package botrest

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/pkg/errors"
	"goji.io"
	"goji.io/pat"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"
)

var serverAddr = ":5002"

func StartServer() {
	muxer := goji.NewMux()
	muxer.Use(dropNonLocal)

	muxer.HandleFunc(pat.Get("/:guild/guild"), HandleGuild)
	muxer.HandleFunc(pat.Get("/:guild/botmember"), HandleBotMember)
	muxer.HandleFunc(pat.Get("/:guild/members"), HandleGetMembers)
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

	http.ListenAndServe(serverAddr, muxer)
}

func ServeJson(w http.ResponseWriter, r *http.Request, data interface{}) {
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logrus.WithError(err).Error("Failed sending json")
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

func dropNonLocal(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Split(r.RemoteAddr, ":")[0] != "127.0.0.1" {
			logrus.Info("Dropped non local connection", r.RemoteAddr)
			return
		}

		inner.ServeHTTP(w, r)
	})
}

func HandleGuild(w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(r, "guild")

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	guild.RLock()
	defer guild.RUnlock()

	gCopy := new(discordgo.Guild)
	*gCopy = *guild.Guild

	gCopy.Members = nil
	gCopy.Presences = nil
	gCopy.VoiceStates = nil

	ServeJson(w, r, gCopy)
}

func HandleBotMember(w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(r, "guild")

	guild := bot.State.Guild(true, gId)
	if guild == nil {
		ServerError(w, r, errors.New("Guild not found"))
		return
	}

	botUser := bot.State.User(true)

	member := guild.MemberCopy(true, botUser.ID, true)
	if member == nil {
		ServerError(w, r, errors.New("Bot Member not found"))
		return
	}

	ServeJson(w, r, member)
}

func HandleGetMembers(w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(r, "guild")
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

	members, _ := bot.GetMembers(gId, uIDs...)

	ServeJson(w, r, members)
}

func HandleChannelPermissions(w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(r, "guild")
	cId := pat.Param(r, "channel")
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
	TotalEvents     int64   `json:"total_events"`
	EventsPerSecond float64 `json:"events_per_second"`

	ConnStatus discordgo.GatewayStatus `json:"conn_status"`

	LastHeartbeatSend time.Time `json:"last_heartbeat_send"`
	LastHeartbeatAck  time.Time `json:"last_heartbeat_ack"`
}

func HandleGWStatus(w http.ResponseWriter, r *http.Request) {

	totalEventStats, periodEventStats := bot.EventLogger.GetStats()

	numShards := bot.ShardManager.GetNumShards()
	result := make([]*ShardStatus, numShards)
	for i := 0; i < numShards; i++ {
		shard := bot.ShardManager.Sessions[i]

		sumEvents := int64(0)
		sumPeriodEvents := int64(0)

		for j, _ := range totalEventStats[i] {
			sumEvents += totalEventStats[i][j]
			sumPeriodEvents += periodEventStats[i][j]
		}

		if shard == nil || shard.GatewayManager == nil {
			result[i] = &ShardStatus{ConnStatus: discordgo.GatewayStatusDisconnected}
			continue
		}

		beat, ack := shard.GatewayManager.HeartBeatStats()

		result[i] = &ShardStatus{
			ConnStatus:        shard.GatewayManager.Status(),
			TotalEvents:       sumEvents,
			EventsPerSecond:   float64(sumPeriodEvents) / bot.EventLoggerPeriodDuration.Seconds(),
			LastHeartbeatSend: beat,
			LastHeartbeatAck:  ack,
		}
	}

	ServeJson(w, r, result)
}

func HandleReconnectShard(w http.ResponseWriter, r *http.Request) {
	sID := pat.Param(r, "shard")
	parsed, _ := strconv.ParseInt(sID, 10, 32)
	shardcount := bot.ShardManager.GetNumShards()
	if parsed < 0 || int(parsed) >= shardcount {
		ServerError(w, r, errors.New("Unknown shard"))
		return
	}

	err := bot.ShardManager.Sessions[parsed].GatewayManager.Reconnect(false)
	if err != nil {
		ServerError(w, r, errors.WithMessage(err, "Reconnect"))
		return
	}

	ServeJson(w, r, "ok")
}
