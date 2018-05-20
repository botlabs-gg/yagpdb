package reputation

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/volatiletech/null.v6"
	"html/template"
	"net/http"
	"strconv"
)

type PostConfigForm struct {
	Enabled                bool
	PointsName             string `valid:",50"`
	Cooldown               int    `valid:"0,86401"` // One day
	MaxGiveAmount          int64
	RequiredGiveRole       string `valid:"role,true"`
	RequiredReceiveRole    string `valid:"role,true"`
	BlacklistedGiveRole    string `valid:"role,true"`
	BlacklistedReceiveRole string `valid:"role,true"`
	AdminRole              string `valid:"role,true"`
}

func (p PostConfigForm) RepConfig() *models.ReputationConfig {
	return &models.ReputationConfig{
		PointsName:             p.PointsName,
		Enabled:                p.Enabled,
		Cooldown:               p.Cooldown,
		MaxGiveAmount:          p.MaxGiveAmount,
		RequiredGiveRole:       null.NewString(p.RequiredGiveRole, true),
		RequiredReceiveRole:    null.NewString(p.RequiredReceiveRole, true),
		BlacklistedGiveRole:    null.NewString(p.BlacklistedGiveRole, true),
		BlacklistedReceiveRole: null.NewString(p.BlacklistedReceiveRole, true),
		AdminRole:              null.NewString(p.AdminRole, true),
	}
}

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/assets/settings.html")))
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/assets/leaderboard.html")))

	subMux := goji.SubMux()
	subMux.Use(web.RequireFullGuildMW)

	web.CPMux.Handle(pat.New("/reputation"), subMux)
	web.CPMux.Handle(pat.New("/reputation/*"), subMux)

	mainGetHandler := web.RenderHandler(HandleGetReputation, "cp_reputation_settings")

	subMux.Handle(pat.Get(""), mainGetHandler)
	subMux.Handle(pat.Get("/"), mainGetHandler)
	subMux.Handle(pat.Post(""), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}, "Updated reputatoin config"))
	subMux.Handle(pat.Post("/"), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}, "Updated reputatoin config"))

	web.ServerPublicMux.Handle(pat.Get("/reputation/leaderboard"), web.RenderHandler(HandleGetReputation, "cp_reputation_leaderboard"))
	web.ServerPubliAPIMux.Handle(pat.Get("/reputation/leaderboard"), web.APIHandler(HandleLeaderboardJson))
}

func HandleGetReputation(w http.ResponseWriter, r *http.Request) interface{} {
	_, activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	if _, ok := templateData["RepSettings"]; !ok {
		settings, err := GetConfig(activeGuild.ID)
		if !web.CheckErr(templateData, err, "Failed retrieving settings", web.CtxLogger(r.Context()).Error) {
			templateData["RepSettings"] = settings
		}
	}

	return templateData
}

func HandlePostReputation(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	_, activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	form := r.Context().Value(common.ContextKeyParsedForm).(*PostConfigForm)
	conf := form.RepConfig()
	conf.GuildID = activeGuild.ID

	templateData["RepSettings"] = conf

	err = conf.UpsertG(true, []string{"guild_id"}, []string{
		"points_name",
		"enabled",
		"cooldown",
		"max_give_amount",
		"required_give_role",
		"required_receive_role",
		"blacklisted_give_role",
		"blacklisted_receive_role",
		"admin_role",
	})

	return
}

func HandleLeaderboardJson(w http.ResponseWriter, r *http.Request) interface{} {
	_, activeGuild, _ := web.GetBaseCPContextData(r.Context())

	conf, err := GetConfig(activeGuild.ID)
	if err != nil {
		return err
	}

	if !conf.Enabled {
		return web.NewPublicError("Reputation not enabled")
	}

	query := r.URL.Query()

	offsetStr := query.Get("offset")
	offset := 0
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).WithField("raw", offsetStr).Error("Failed parsing offset")
		}
	}

	limitStr := query.Get("limit")
	limit := 0
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).WithField("raw", limitStr).Error("Failed parsing limit")
		}
	}

	if limit > 100 || limit < 0 {
		limit = 10
	}

	top, err := TopUsers(activeGuild.ID, offset, limit)
	if err != nil {
		return err
	}

	entries, err := DetailedLeaderboardEntries(activeGuild.ID, top)
	if err != nil {
		return err
	}

	return entries
}
