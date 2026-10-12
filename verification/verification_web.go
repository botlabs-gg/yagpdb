package verification

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/verification/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/botlabs-gg/yagpdb/v2/web/discorddata"
	"github.com/russross/blackfriday"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"goji.io/pat"
)

//go:embed assets/verification_control_panel.html
var PageHTMLControlPanel string

//go:embed assets/verification_verify_page.html
var PageHTMLVerifyPage string

type FormData struct {
	Enabled             bool
	VerifiedRole        int64  `valid:"role,true"`
	PageContent         string `valid:",10000"`
	KickUnverifiedAfter int    `valid:"0,"`
	WarnUnverifiedAfter int    `valid:"0,"`
	WarnMessage         string `valid:"template,10000"`
	DMMessage           string `valid:"template,10000"`
	LogChannel          int64  `valid:"channel,true"`
}

var panelLogKey = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "verification_updated_settings", FormatString: "Updated verification settings"})

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("verification/assets/verification_control_panel.html", PageHTMLControlPanel)
	web.AddHTMLTemplate("verification/assets/verification_verify_page.html", PageHTMLVerifyPage)

	web.AddSidebarItem(web.SidebarCategoryModeration, &web.SidebarItem{
		Name: "Verification",
		URL:  "verification",
		Icon: "fas fa-address-card",
	})

	getHandler := web.ControllerHandler(p.handleGetSettings, "cp_verification_settings")
	postHandler := web.ControllerPostHandler(p.handlePostSettings, getHandler, FormData{})

	web.CPMux.Handle(pat.Get("/verification"), web.RequireBotMemberMW(getHandler))
	web.CPMux.Handle(pat.Get("/verification/"), web.RequireBotMemberMW(getHandler))

	web.CPMux.Handle(pat.Post("/verification"), postHandler)

	previewHandler := web.ControllerHandler(p.handlePreviewVerifyPage, "verification_verify_page")
	web.CPMux.Handle(pat.Get("/verification/preview"), previewHandler)
	web.CPMux.Handle(pat.Post("/verification/preview"), previewHandler)

	getVerifyPageHandler := web.ControllerHandler(p.handleGetVerifyPage, "verification_verify_page")
	postVerifyPageHandler := web.ControllerPostHandler(p.handlePostVerifyPage, getVerifyPageHandler, nil)
	web.RootMux.Handle(pat.Get("/v/:token"), getVerifyPageHandler)
	web.RootMux.Handle(pat.Post("/v/:token"), postVerifyPageHandler)

	// The long form is in dms sent before the short one existed.
	web.ServerPublicMux.Handle(pat.Get("/verify/:user_id/:token"), getVerifyPageHandler)
	web.ServerPublicMux.Handle(pat.Post("/verify/:user_id/:token"), postVerifyPageHandler)
}

// VerifyPageURL is the link a user follows to verify. The token alone
// identifies the session, so the link carries nothing else.
func VerifyPageURL(token string) string {
	return fmt.Sprintf("%s/v/%s", web.BaseURL(), token)
}

func findOpenVerificationSession(ctx context.Context, token string) (*models.VerificationSession, error) {
	return models.VerificationSessions(
		models.VerificationSessionWhere.Token.EQ(token),
		models.VerificationSessionWhere.ExpiredAt.IsNull(),
		models.VerificationSessionWhere.SolvedAt.IsNull()).OneG(ctx)
}

func findVerificationSettings(ctx context.Context, guildID int64) (*models.VerificationConfig, error) {
	settings, err := models.FindVerificationConfigG(ctx, guildID)
	if err == sql.ErrNoRows {
		return &models.VerificationConfig{GuildID: guildID}, nil
	}

	return settings, err
}

const noVerificationSessionMessage = "This verification link has expired or was already used. Try rejoining the server, or contact its staff if the problem persists."

func (p *Plugin) handleGetSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, templateData := web.GetBaseCPContextData(ctx)

	settings, err := findVerificationSettings(ctx, g.ID)
	if err != nil {
		return templateData, err
	}

	roleInvalid := true
	for _, role := range g.Roles {
		if role.ID == settings.VerifiedRole {
			roleInvalid = false
			break
		}
	}

	if settings.DMMessage == "" {
		settings.DMMessage = DefaultDMMessage
	}

	settings.PageContent = pageContentOrDefault(settings.PageContent)
	templateData["DefaultPageContent"] = DefaultPageContent
	templateData["PluginSettings"] = settings
	templateData["RoleInvalid"] = roleInvalid

	return templateData, nil
}

func (p *Plugin) handlePostSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, templateData := web.GetBaseCPContextData(ctx)

	formConfig := ctx.Value(common.ContextKeyParsedForm).(*FormData)

	model := &models.VerificationConfig{
		GuildID:             g.ID,
		Enabled:             formConfig.Enabled,
		VerifiedRole:        formConfig.VerifiedRole,
		PageContent:         formConfig.PageContent,
		KickUnverifiedAfter: formConfig.KickUnverifiedAfter,
		WarnUnverifiedAfter: formConfig.WarnUnverifiedAfter,
		WarnMessage:         formConfig.WarnMessage,
		LogChannel:          formConfig.LogChannel,
		DMMessage:           formConfig.DMMessage,
	}

	columns := boil.Whitelist("enabled", "verified_role", "page_content", "kick_unverified_after", "warn_unverified_after", "warn_message", "log_channel", "dm_message")
	columnsCreate := boil.Whitelist("guild_id", "enabled", "verified_role", "page_content", "kick_unverified_after", "warn_unverified_after", "warn_message", "log_channel", "dm_message")
	err := model.UpsertG(ctx, true, []string{"guild_id"}, columns, columnsCreate)
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKey))
	}
	return templateData, err
}

// handleGetVerifyPage renders a standalone page rather than the dashboard, as
// the user only needs to solve the captcha, not log in or see the panel.
func (p *Plugin) handleGetVerifyPage(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, templateData := web.GetCreateTemplateData(ctx)

	if _, ok := templateData["REValid"]; ok {
		// Just submitted, the session is solved by now so there is nothing to look up.
		return templateData, nil
	}

	session, err := findOpenVerificationSession(ctx, pat.Param(r, "token"))
	if err == sql.ErrNoRows {
		templateData.AddAlerts(web.ErrorAlert(noVerificationSessionMessage))
		return templateData, nil
	}
	if err != nil {
		return templateData, err
	}

	settings, err := findVerificationSettings(ctx, session.GuildID)
	if err != nil {
		return templateData, err
	}

	if gs, err := discorddata.GetFullGuild(session.GuildID); err == nil && gs != nil {
		templateData["GuildName"] = gs.Name
	}

	if !settings.Enabled {
		templateData.AddAlerts(web.ErrorAlert("Verification system disabled on this server"))
		return templateData, nil
	}

	templateData["GoogleReCaptchaSiteKey"] = confGoogleReCAPTCHASiteKey.GetString()

	templateData["RenderedPageContent"] = renderPageContent(settings.PageContent)

	return templateData, nil
}

func renderPageContent(content string) template.HTML {
	unsafe := blackfriday.MarkdownCommon([]byte(pageContentOrDefault(content)))
	return template.HTML(common.UGCHtmlPolicy.SanitizeBytes(unsafe))
}

// handlePreviewVerifyPage shows the verify page as users will see it, from the
// unsaved content when posted from the editor. The captcha is not loaded, so a
// preview never counts against the reCAPTCHA quota.
func (p *Plugin) handlePreviewVerifyPage(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, templateData := web.GetBaseCPContextData(ctx)

	content := r.FormValue("PageContent")
	if r.Method != http.MethodPost {
		settings, err := findVerificationSettings(ctx, g.ID)
		if err != nil {
			return templateData, err
		}
		content = settings.PageContent
	}

	templateData["Preview"] = true
	templateData["GuildName"] = g.Name
	templateData["RenderedPageContent"] = renderPageContent(content)

	return templateData, nil
}

func (p *Plugin) handlePostVerifyPage(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, templateData := web.GetCreateTemplateData(ctx)

	verSession, err := findOpenVerificationSession(ctx, pat.Param(r, "token"))
	if err == sql.ErrNoRows {
		templateData.AddAlerts(web.ErrorAlert(noVerificationSessionMessage))
		return templateData, nil
	}
	if err != nil {
		return templateData, err
	}

	settings, err := findVerificationSettings(ctx, verSession.GuildID)
	if err != nil {
		return templateData, err
	}

	if !settings.Enabled {
		templateData.AddAlerts(web.ErrorAlert("Verification system disabled on this server"))
		return templateData, nil
	}

	valid, err := p.checkCAPTCHAResponse(r.FormValue("g-recaptcha-response"))
	if err != nil {
		logrus.WithError(err).Error("Failed recaptcha response")
	}

	if !valid {
		templateData.AddAlerts(web.ErrorAlert("Invalid reCAPTCHA submission."))
		return templateData, nil
	}

	ip := ""
	if confVerificationTrackIPs.GetBool() {
		ip = web.GetRequestIP(r)
	}

	model := &models.VerifiedUser{
		UserID:     verSession.UserID,
		GuildID:    verSession.GuildID,
		VerifiedAt: time.Now(),
		IP:         ip,
	}

	err = model.UpsertG(ctx, true, []string{"guild_id", "user_id"}, boil.Infer(), boil.Infer())
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("failed verifying user")
		return templateData, err
	}

	scheduledevents2.ScheduleEvent("verification_user_verified", verSession.GuildID, time.Now(), verSession.UserID)
	verSession.SolvedAt = null.TimeFrom(time.Now())
	verSession.UpdateG(ctx, boil.Infer())

	go analytics.RecordActiveUnit(verSession.GuildID, p, "completed")

	if gs, err := discorddata.GetFullGuild(verSession.GuildID); err == nil && gs != nil {
		templateData["GuildName"] = gs.Name
	}
	templateData["REValid"] = true

	return templateData, nil
}

type CheckCAPTCHAResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

type CheckCAPTCHARequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

func (p *Plugin) checkCAPTCHAResponse(response string) (valid bool, err error) {

	v := url.Values{
		"response": {response},
		"secret":   {confGoogleReCAPTCHASecret.GetString()},
	}

	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", v)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	var dst CheckCAPTCHAResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&dst)
	if err != nil {
		return false, err
	}

	if !dst.Success {
		logger.Warnf("reCAPTCHA failed: %#v", dst)
	}

	return dst.Success, nil
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())
	ctx := r.Context()

	templateData["WidgetTitle"] = "Google reCAPTCHA Verification"
	templateData["SettingsPath"] = "/verification"

	settings, err := models.FindVerificationConfigG(ctx, ag.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			settings = &models.VerificationConfig{
				GuildID: ag.ID,
			}
		} else {
			return templateData, err
		}
	}

	format := `<ul>
	<li>Status: %s</li>
	<li>Role: <code>%s</code> %s</li>
</ul>`

	status := web.EnabledDisabledSpanStatus(settings.Enabled)

	if settings.Enabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	roleStr := "none / unknown"
	indicatorRole := ""
	if role := ag.GetRole(settings.VerifiedRole); role != nil {
		roleStr = html.EscapeString(role.Name)
		indicatorRole = web.Indicator(true)
	} else {
		indicatorRole = web.Indicator(false)
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, status, roleStr, indicatorRole))

	return templateData, nil
}
