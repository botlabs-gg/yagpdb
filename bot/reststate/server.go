package reststate

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"net/http"
	"strings"
)

var serverAddr = ":5002"

func StartServer() {
	muxer := goji.NewMux()
	muxer.UseC(dropNonLocal)

	muxer.HandleFuncC(pat.Get("/:guild/guild"), HandleGuild)
	muxer.HandleFuncC(pat.Get("/:guild/botmember"), HandleBotMember)

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

func dropNonLocal(inner goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if strings.Split(r.RemoteAddr, ":")[0] != "127.0.0.1" {
			logrus.Info("Dropped non local connection", r.RemoteAddr)
			return
		}

		inner.ServeHTTPC(ctx, w, r)
	})

}

func HandleGuild(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(ctx, "guild")

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

func HandleBotMember(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	gId := pat.Param(ctx, "guild")

	member, err := common.BotSession.State.Member(gId, common.BotSession.State.User.ID)
	if ServerError(w, r, err) {
		return
	}

	ServeJson(w, r, member)
}
