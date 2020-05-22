package autorole

import (
	"net/http"
	"strconv"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/internalapi"
	"github.com/mediocregopher/radix/v3"
	"goji.io"
	"goji.io/pat"
)

var _ internalapi.InternalAPIPlugin = (*Plugin)(nil)
var ErrAlreadyProcessingFullGuild = errors.New("Already processing users on this guild")

func (p *Plugin) InitInternalAPIRoutes(mux *goji.Mux) {
	mux.Handle(pat.Post("/:guild/autorole/fullscan"), http.HandlerFunc(botRestHandleScanFullServer))
}

func botRestHandleScanFullServer(w http.ResponseWriter, r *http.Request) {
	guildID := pat.Param(r, "guild")
	parsedGID, _ := strconv.ParseInt(guildID, 10, 64)

	if parsedGID == 0 {
		internalapi.ServerError(w, r, errors.New("unknown server"))
		return
	}

	logger.WithField("guild", parsedGID).Info("autorole doing a full scan")
	session := bot.ShardManager.SessionForGuild(parsedGID)
	session.GatewayManager.RequestGuildMembers(parsedGID, "", 0)

	internalapi.ServeJson(w, r, "ok")
}

func botRestPostFullScan(guildID int64) error {
	var resp string
	err := common.RedisPool.Do(radix.Cmd(&resp, "SET", RedisKeyGuildChunkProecssing(guildID), "1", "EX", "10", "NX"))
	if err != nil {
		return errors.WithMessage(err, "r.SET")
	}

	if resp != "OK" {
		return ErrAlreadyProcessingFullGuild
	}

	err = internalapi.PostWithGuild(guildID, strconv.FormatInt(guildID, 10)+"/autorole/fullscan", nil, nil)
	return err
}
