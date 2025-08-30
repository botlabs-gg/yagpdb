package bulkrole

import (
	"net/http"
	"strconv"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"goji.io"
	"goji.io/pat"
)

var _ internalapi.InternalAPIPlugin = (*Plugin)(nil)

func (p *Plugin) InitInternalAPIRoutes(mux *goji.Mux) {
	mux.Handle(pat.Post("/:guild/bulkrole/start"), http.HandlerFunc(botRestHandleStartOperation))
	mux.Handle(pat.Post("/:guild/bulkrole/cancel"), http.HandlerFunc(botRestHandleCancelOperation))
	mux.Handle(pat.Get("/:guild/bulkrole/status"), http.HandlerFunc(botRestHandleGetStatus))
}

func botRestHandleStartOperation(w http.ResponseWriter, r *http.Request) {
	guildID := pat.Param(r, "guild")
	parsedGID, _ := strconv.ParseInt(guildID, 10, 64)

	if parsedGID == 0 {
		internalapi.ServerError(w, r, errors.New("unknown server"))
		return
	}

	config, err := GetBulkRoleConfig(parsedGID)
	if err != nil {
		logger.WithField("guild", parsedGID).WithError(err).Error("failed to get config via internal API")
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to get config"))
		return
	}

	err = config.startBulkRoleOperation()
	if err != nil {
		logger.WithField("guild", parsedGID).WithError(err).Error("failed to start bulk role operation via internal API")
		internalapi.ServerError(w, r, err)
		return
	}

	logger.WithField("guild", parsedGID).Info("bulkrole operation started via internal API")
	internalapi.ServeJson(w, r, "ok")
}

func botRestHandleCancelOperation(w http.ResponseWriter, r *http.Request) {
	guildID := pat.Param(r, "guild")
	parsedGID, _ := strconv.ParseInt(guildID, 10, 64)

	if parsedGID == 0 {
		internalapi.ServerError(w, r, errors.New("unknown server"))
		return
	}

	config, err := GetBulkRoleConfig(parsedGID)
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to get config"))
		return
	}

	err = config.cancelBulkRoleOperation()
	if err != nil {
		internalapi.ServerError(w, r, err)
		return
	}

	logger.WithField("guild", parsedGID).Info("bulkrole operation cancelled via internal API")
	internalapi.ServeJson(w, r, "ok")
}

func botRestHandleGetStatus(w http.ResponseWriter, r *http.Request) {
	guildID := pat.Param(r, "guild")
	parsedGID, _ := strconv.ParseInt(guildID, 10, 64)

	if parsedGID == 0 {
		internalapi.ServerError(w, r, errors.New("unknown server"))
		return
	}

	config, err := GetBulkRoleConfig(parsedGID)
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to get config"))
		return
	}

	status, processed, results, err := config.getBulkRoleStatus()
	if err != nil {
		internalapi.ServerError(w, r, err)
		return
	}

	response := StatusResponse{
		Status:    status,
		Processed: processed,
		Results:   results,
	}

	internalapi.ServeJson(w, r, response)
}
