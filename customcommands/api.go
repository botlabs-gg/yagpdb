package customcommands

import (
	"net/http"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"

	"goji.io/pat"
)

var (
	confCCAPIEnabled        = config.RegisterOption("yagpdb.custom_command_api.enabled", "Enable custom command management API", false)
	confCCAPIRequirePremium = config.RegisterOption("yagpdb.custom_command_api.require_premium", "Require guild to be premium for API access", true)
)

func (p *Plugin) initAPI() {
	if !confCCAPIEnabled.GetBool() {
		logger.Warn("Custom commands API disabled, not starting it")
		return
	}

	logger.Info("Starting custom commands API")

	subMux := goji.SubMux()
	subMux.Use(canUseAPIMW)

	// TODO: This needs better middleware, during development we don't care right now
	subMux.Use(web.RequireActiveServer)

	web.ServerPublicAPIMux.Handle(pat.New("/customcommands"), subMux)
	web.ServerPublicAPIMux.Handle(pat.New("/customcommands/*"), subMux)

	subMux.Handle(pat.Get(""), web.APIHandler(handleGetAllCustomCommands))
	subMux.Handle(pat.Get("/"), web.APIHandler(handleGetAllCustomCommands))

	subMux.Handle(pat.Get("/:id"), web.APIHandler(handleGetCustomCommand))
	subMux.Handle(pat.Get("/:id/"), web.APIHandler(handleGetCustomCommand))
}

// TODO: implement authorisation middleware

func canUseAPIMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		if !confCCAPIRequirePremium.GetBool() {
			// serve as normal, no premium required
			inner.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		activeGuild, _ := web.GetBaseCPContextData(ctx)

		isPremium, err := premium.IsGuildPremium(activeGuild.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !isPremium {
			http.Error(w, "Guild does not have premium.", http.StatusForbidden)
			return
		}
	}

	return http.HandlerFunc(mw)
}

func handleGetAllCustomCommands(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	ccs, err := models.CustomCommands(qm.Where("guild_id = ?", activeGuild.ID)).AllG(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}

	return ccs
}

func handleGetCustomCommand(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	ccID, err := strconv.ParseInt(pat.Param(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}

	cc, err := models.CustomCommands(
		models.CustomCommandWhere.GuildID.EQ(activeGuild.ID),
		models.CustomCommandWhere.LocalID.EQ(ccID)).OneG(r.Context())

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}

	return cc
}

// TODO: implement POST new CC

// TODO: implement PATCH CC

// TODO: implement DELETE CC
