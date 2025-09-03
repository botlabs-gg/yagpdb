package reputation

import (
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/reputation/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/reputation_leaderboard.html
var PageHTMLLeaderboard string

//go:embed assets/reputation_settings.html
var PageHTMLSettings string

type PostConfigForm struct {
	Enabled                   bool
	EnableThanksDetection     bool
	PointsName                string  `valid:",50"`
	Cooldown                  int     `valid:"0,86401"` // One day
	MaxGiveAmount             int64   `valid:"1,"`
	MaxRemoveAmount           int64   `valid:"1,"`
	RequiredGiveRoles         []int64 `valid:"role,true"`
	RequiredReceiveRoles      []int64 `valid:"role,true"`
	BlacklistedGiveRoles      []int64 `valid:"role,true"`
	BlacklistedReceiveRoles   []int64 `valid:"role,true"`
	AdminRoles                []int64 `valid:"role,true"`
	WhitelistedThanksChannels []int64 `valid:"channel,true"`
	BlacklistedThanksChannels []int64 `valid:"channel,true"`
	ThanksRegex               string  `valid:"regex,2000"`
}

func (p PostConfigForm) RepConfig() *models.ReputationConfig {
	return &models.ReputationConfig{
		PointsName:                p.PointsName,
		Enabled:                   p.Enabled,
		Cooldown:                  p.Cooldown,
		MaxGiveAmount:             p.MaxGiveAmount,
		MaxRemoveAmount:           p.MaxRemoveAmount,
		RequiredGiveRoles:         p.RequiredGiveRoles,
		RequiredReceiveRoles:      p.RequiredReceiveRoles,
		BlacklistedGiveRoles:      p.BlacklistedGiveRoles,
		BlacklistedReceiveRoles:   p.BlacklistedReceiveRoles,
		AdminRoles:                p.AdminRoles,
		DisableThanksDetection:    !p.EnableThanksDetection,
		WhitelistedThanksChannels: p.WhitelistedThanksChannels,
		BlacklistedThanksChannels: p.BlacklistedThanksChannels,
		ThanksRegex:               null.String{String: p.ThanksRegex, Valid: p.ThanksRegex != ""},
	}
}

type NewRoleForm struct {
	Threshold int64 `valid:"1,"`
	Role      int64 `valid:"role,true"`
}

type PostRoleForm struct {
	ID   int64
	Role int64 `valid:"role,true"`
}

var (
	panelLogKeyUpdatedSettings = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "reputation_settings_updated", FormatString: "Updated reputation settings"})
	panelLogKeyResetReputation = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "reputation_reset_reputation", FormatString: "Reset reputation"})
	panelLogKeyNewRepRole      = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "reputation_role_added", FormatString: "Reputation role created"})
	panelLogKeyUpdateRepRole   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "reputation_role_updated", FormatString: "Reputation role updated"})
	panelLogKeyDeleteRepRole   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "reputation_role_deleted", FormatString: "Reputation role deleted"})
)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("reputation/assets/reputation_settings.html", PageHTMLSettings)
	web.AddHTMLTemplate("reputation/assets/reputation_leaderboard.html", PageHTMLLeaderboard)
	web.AddSidebarItem(web.SidebarCategoryFun, &web.SidebarItem{
		Name: "Reputation",
		URL:  "reputation",
		Icon: "fas fa-angry",
	})

	subMux := goji.SubMux()

	web.CPMux.Handle(pat.New("/reputation"), subMux)
	web.CPMux.Handle(pat.New("/reputation/*"), subMux)

	subMux.Use(web.RequireBotMemberMW)

	mainGetHandler := web.RenderHandler(HandleGetReputation, "cp_reputation_settings")

	subMux.Handle(pat.Get(""), mainGetHandler)
	subMux.Handle(pat.Get("/"), mainGetHandler)
	subMux.Handle(pat.Post(""), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}))
	subMux.Handle(pat.Post("/"), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}))
	subMux.Handle(pat.Post("/reset_users"), web.ControllerPostHandler(HandleResetReputation, mainGetHandler, nil))
	subMux.Handle(pat.Get("/logs"), web.APIHandler(HandleLogsJson))

	subMux.Handle(pat.Post("/new_role"), web.ControllerPostHandler(HandleNewRepRole, mainGetHandler, NewRoleForm{}))
	subMux.Handle(pat.Post("/roles/:id/update"), web.ControllerPostHandler(HandleUpdateRepRole, mainGetHandler, PostRoleForm{}))
	subMux.Handle(pat.Post("/roles/:id/delete"), web.ControllerPostHandler(HandleDeleteRepRole, mainGetHandler, nil))

	web.ServerPublicMux.Handle(pat.Get("/reputation/leaderboard"), web.RenderHandler(HandleGetReputation, "cp_reputation_leaderboard"))
	web.ServerPublicAPIMux.Handle(pat.Get("/reputation/leaderboard"), web.APIHandler(HandleLeaderboardJson))
}

func HandleGetReputation(w http.ResponseWriter, r *http.Request) interface{} {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	if _, ok := templateData["RepSettings"]; !ok {
		settings, err := GetConfig(r.Context(), activeGuild.ID)
		if !web.CheckErr(templateData, err, "Failed retrieving settings", web.CtxLogger(r.Context()).Error) {
			templateData["RepSettings"] = settings
		}
	}

	repRoles, err := models.ReputationRoles(
		models.ReputationRoleWhere.GuildID.EQ(activeGuild.ID),
		qm.OrderBy("rep_threshold ASC"),
	).AllG(r.Context())
	if !web.CheckErr(templateData, err, "Failed retrieving reputation roles", web.CtxLogger(r.Context()).Error) {
		templateData["RepRoles"] = repRoles
	}
	return templateData
}

func HandlePostReputation(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	form := r.Context().Value(common.ContextKeyParsedForm).(*PostConfigForm)
	conf := form.RepConfig()
	conf.GuildID = activeGuild.ID

	// Premium-only: custom thanks regex
	if !premium.ContextPremium(r.Context()) {
		conf.ThanksRegex = null.String{}
	}

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
		"whitelisted_thanks_channels",
		"blacklisted_thanks_channels",
		"thanks_regex",
	), boil.Infer())

	if err == nil {
		featureflags.MarkGuildDirty(activeGuild.ID)
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedSettings))
	}

	return
}

func HandleResetReputation(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	_, err = models.ReputationUsers(qm.Where("guild_id = ?", activeGuild.ID)).DeleteAll(r.Context(), common.PQ)
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyResetReputation))
	}
	return templateData, err
}

func HandleNewRepRole(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	form := r.Context().Value(common.ContextKeyParsedForm).(*NewRoleForm)

	existing, err := models.ReputationRoles(models.ReputationRoleWhere.GuildID.EQ(activeGuild.ID)).AllG(r.Context())
	if err != nil {
		return templateData, err
	}

	if lim := GuildMaxRepRoles(activeGuild.ID); len(existing) > lim {
		return templateData.AddAlerts(web.ErrorAlert("Too many rep roles (max ", lim, ")")), nil
	}
	for _, r := range existing {
		if r.RepThreshold == form.Threshold {
			return templateData.AddAlerts(web.ErrorAlert("Already exists rep role with that threshold")), nil
		}
	}

	repRole := &models.ReputationRole{
		GuildID:      activeGuild.ID,
		RepThreshold: form.Threshold,
		Role:         form.Role,
	}
	err = repRole.InsertG(r.Context(), boil.Infer())
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyNewRepRole))
	}
	return templateData, err
}

func HandleUpdateRepRole(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	form := r.Context().Value(common.ContextKeyParsedForm).(*PostRoleForm)

	repRole, err := models.FindReputationRoleG(r.Context(), form.ID)
	if err != nil {
		return templateData, err
	}
	if repRole.GuildID != activeGuild.ID {
		return templateData.AddAlerts(web.ErrorAlert("Cannot edit rep role from other server")), nil
	}

	repRole.Role = form.Role
	_, err = repRole.UpdateG(r.Context(), boil.Infer())
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdateRepRole))
	}
	return templateData, err
}

func HandleDeleteRepRole(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reputation"

	id, err := strconv.ParseInt(pat.Param(r, "id"), 10, 64)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Invalid rep role")), nil
	}

	rowsAff, err := models.ReputationRoles(
		models.ReputationRoleWhere.GuildID.EQ(activeGuild.ID),
		models.ReputationRoleWhere.ID.EQ(id),
	).DeleteAllG(r.Context())
	if err != nil {
		return templateData, err
	}

	if rowsAff > 0 {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyDeleteRepRole))
	}
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
