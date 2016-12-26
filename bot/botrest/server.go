package botrest

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"goji.io"
	"goji.io/pat"
	"net/http"
	"net/http/pprof"
	"strings"
)

var serverAddr = ":5002"

func StartServer() {
	muxer := goji.NewMux()
	muxer.Use(dropNonLocal)

	muxer.HandleFunc(pat.Get("/:guild/guild"), HandleGuild)
	muxer.HandleFunc(pat.Get("/:guild/botmember"), HandleBotMember)
	muxer.HandleFunc(pat.Get("/ping"), HandlePing)

	// Debug stuff
	muxer.HandleFunc(pat.Get("/debug/pprof/other/*"), pprof.Index)
	muxer.HandleFunc(pat.Get("/debug/pprof/cmdline"), pprof.Cmdline)
	muxer.HandleFunc(pat.Get("/debug/pprof/profile"), pprof.Profile)
	muxer.HandleFunc(pat.Get("/debug/pprof/symbol"), pprof.Symbol)
	muxer.HandleFunc(pat.Get("/debug/pprof/trace"), pprof.Trace)

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

	w.WriteHeader(http.StatusInternalServerError)
	w.Write(nil)
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

	guild, err := common.BotSession.State.Guild(gId)
	if ServerError(w, r, err) {
		return
	}

	gCopy := *guild
	gCopy.Members = nil
	gCopy.Presences = nil
	gCopy.VoiceStates = nil

	ServeJson(w, r, gCopy)
}

func HandleBotMember(w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(r, "guild")

	member, err := common.BotSession.State.Member(gId, common.BotSession.State.User.ID)
	if ServerError(w, r, err) {
		return
	}

	ServeJson(w, r, member)
}

func HandlePing(w http.ResponseWriter, r *http.Request) {
	ServeJson(w, r, "pong")
}
