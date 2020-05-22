package reputation

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

type PostConfigForm struct {
	Enabled                 bool
	EnableThanksDetection   bool
	PointsName              string `valid:",50"`
	Cooldown                int    `valid:"0,86401"` // One day
	MaxGiveAmount           int64
	MaxRemoveAmount         int64
	RequiredGiveRoles       []int64 `valid:"role,true"`
	RequiredReceiveRoles    []int64 `valid:"role,true"`
	BlacklistedGiveRoles    []int64 `valid:"role,true"`
	BlacklistedReceiveRoles []int64 `valid:"role,true"`
	AdminRoles              []int64 `valid:"role,true"`
}

func (p PostConfigForm) RepConfig() *models.ReputationConfig {
	return &models.ReputationConfig{
		PointsName:              p.PointsName,
		Enabled:                 p.Enabled,
		Cooldown:                p.Cooldown,
		MaxGiveAmount:           p.MaxGiveAmount,
		MaxRemoveAmount:         p.MaxRemoveAmount,
		RequiredGiveRoles:       p.RequiredGiveRoles,
		RequiredReceiveRoles:    p.RequiredReceiveRoles,
		BlacklistedGiveRoles:    p.BlacklistedGiveRoles,
		BlacklistedReceiveRoles: p.BlacklistedReceiveRoles,
		AdminRoles:              p.AdminRoles,
		DisableThanksDetection:  !p.EnableThanksDetection,
	}
}

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../reputation/assets/reputation_settings.html", "templates/plugins/reputation_settings.html")
	web.LoadHTMLTemplate("../../reputation/assets/reputation_leaderboard.html", "templates/plugins/reputation_leaderboard.html")
	web.AddSidebarItem(web.SidebarCategoryFun, &web.SidebarItem{
		Name: "Reputation",
		URL:  "reputation",
		Icon: "fas fa-angry",
	})

	subMux := goji.SubMux()

	web.CPMux.Handle(pat.New("/reputation"), subMux)
	web.CPMux.Handle(pat.New("/reputation/*"), subMux)

	mainGetHandler := web.RenderHandler(HandleGetReputation, "cp_reputation_settings")

	subMux.Handle(pat.Get(""), mainGetHandler)
	subMux.Handle(pat.Get("/"), mainGetHandler)
	subMux.Handle(pat.Post(""), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}, "Updated reputation config"))
	subMux.Handle(pat.Post("/"), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}, "Updated reputation config"))
	subMux.Handle(pat.Post("/reset_users"), web.ControllerPostHandler(HandleResetReputation, mainGetHandler, nil, "Reset reputation"))
	subMux.Handle(pat.Get("/logs"), web.APIHandler(HandleLogsJson))

	web.ServerPublicMux.Handle(pat.Get("/reputation/leaderboard"), web.RenderHandler(HandleGetReputation, "cp_reputation_leaderboard"))
	web.ServerPubliAPIMux.Handle(pat.Get("/reputation/leaderboard"), web.APIHandler(HandleLeaderboardJson))
}

func HandleGetReputation(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	if _, ok := templateData["RepSettings"]; !ok {
		settings, err := GetConfig(r.Context(), activeGuild.ID)
		if !web.CheckErr(templateData, err, "Failed retrieving settings", web.CtxLogger(r.Context()).Error) {
			templateData["RepSettings"] = settings
		}
	}

	return templateData
}

func HandlePostReputation(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	form := r.Context().Value(common.ContextKeyParsedForm).(*PostConfigForm)
	conf := form.RepConfig()
	conf.GuildID = activeGuild.ID

	templateData["RepSettings"] = conf

	err = conf.UpsertG(r.Context(), true, []string{"guild_id"}, boil.Whitelist(
		"points_name",
		"enabled",
		"cooldown",
		"max_give_amount",
		"max_remove_amount",
		"required_give_roles",
		"required_receive_roles",
		"blacklisted_give_roles",
		"blacklisted_receive_roles",
		"admin_roles",
		"disable_thanks_detection",
	), boil.Infer())

	if err == nil {
		featureflags.MarkGuildDirty(activeGuild.ID)
	}

	return
}

func HandleResetReputation(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	_, err = models.ReputationUsers(qm.Where("guild_id = ?", activeGuild.ID)).DeleteAll(r.Context(), common.PQ)
	return templateData, err
}

func HandleLeaderboardJson(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	conf, err := GetConfig(r.Context(), activeGuild.ID)
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

func HandleLogsJson(W http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	query := r.URL.Query()
	after, _ := strconv.ParseInt(query.Get("after"), 10, 64)
	before, _ := strconv.ParseInt(query.Get("before"), 10, 64)

	// usernameQuery := query.Get("username")
	idQuery, _ := strconv.ParseInt(query.Get("user_id"), 10, 64)

	var result []*models.ReputationLog

	if idQuery == 0 {
		return result
	}

	clauses := make([]qm.QueryMod, 4, 5)
	clauses[0] = qm.Where("guild_id = ?", activeGuild.ID)
	clauses[1] = qm.Where("(receiver_id = ? OR sender_id = ?)", idQuery, idQuery)
	clauses[2] = qm.OrderBy("id desc")
	clauses[3] = qm.Limit(100)

	if after != 0 {
		clauses = append(clauses, qm.Where("id > ?", after))
	} else if before != 0 {
		clauses = append(clauses, qm.Where("id < ?", before))
	}

	result, err := models.ReputationLogs(clauses...).AllG(r.Context())
	if err != nil {
		return err
	}

	return result
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Reputation"
	templateData["SettingsPath"] = "/reputation"

	settings, err := GetConfig(r.Context(), ag.ID)
	if err != nil {
		return templateData, err
	}

	const format = `<ul>
	<li>Reputation is: %s</li>
	<li>Reputation name: <code>%s</code></li>
</ul>`

	name := html.EscapeString(settings.PointsName)
	if settings.Enabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, web.EnabledDisabledSpanStatus(settings.Enabled), name))

	return templateData, nil
}
