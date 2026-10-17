package moderation

import (
	_ "embed"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"goji.io/pat"
)

//go:embed assets/moderation_ban_appeal.html
var banAppealPageHTML string

type banAppealPageState string

const (
	banAppealPageDisabled  banAppealPageState = "disabled"
	banAppealPageNotBanned banAppealPageState = "not_banned"
	banAppealPagePending   banAppealPageState = "pending"
	banAppealPageForm      banAppealPageState = "form"
	banAppealPageSubmitted banAppealPageState = "submitted"
)

type BanAppealForm struct {
	Appeal string
}

func initBanAppealWeb() {
	web.AddHTMLTemplate("moderation/assets/moderation_ban_appeal.html", banAppealPageHTML)

	getHandler := web.RequireSessionMiddleware(web.ControllerHandler(handleGetBanAppealPage, "moderation_ban_appeal"))
	postHandler := web.RequireSessionMiddleware(web.ControllerPostHandler(handlePostBanAppeal, getHandler, BanAppealForm{}))

	submitLimiter := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	submitLimiter.SetIPLookups([]string{"CF-Connecting-IP", "X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	web.ServerPublicMux.Handle(pat.Get("/appeal"), getHandler)
	web.ServerPublicMux.Handle(pat.Post("/appeal"), tollbooth.LimitHandler(submitLimiter, postHandler))
}

func handleGetBanAppealPage(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	guild, templateData := web.GetBaseCPContextData(ctx)
	templateData["GuildName"] = guild.Name
	templateData["BanAppealMinLength"] = BanAppealMinLength
	templateData["BanAppealMaxLength"] = BanAppealMaxLength

	if _, alreadyDecided := templateData["AppealState"]; alreadyDecided {
		return templateData, nil
	}

	config, err := FetchConfig(guild.ID)
	if err != nil {
		return templateData, err
	}

	if !config.BanAppealsOpen() {
		templateData["AppealState"] = banAppealPageDisabled
		return templateData, nil
	}

	user := web.ContextUser(ctx)
	open, err := FindOpenBanAppeal(ctx, guild.ID, user.ID)
	if err != nil {
		return templateData, err
	}
	if open != nil {
		templateData["AppealState"] = banAppealPagePending
		templateData["AppealedAt"] = open.CreatedAt
		return templateData, nil
	}

	ban, err := CurrentBan(guild.ID, user.ID)
	if err != nil {
		return templateData, err
	}
	if ban == nil {
		templateData["AppealState"] = banAppealPageNotBanned
		return templateData, nil
	}

	templateData["AppealState"] = banAppealPageForm
	templateData["BanReason"] = ban.Reason
	return templateData, nil
}

func handlePostBanAppeal(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	guild, templateData := web.GetBaseCPContextData(ctx)

	appealText := strings.TrimSpace(ctx.Value(common.ContextKeyParsedForm).(*BanAppealForm).Appeal)
	if length := utf8.RuneCountInString(appealText); length < BanAppealMinLength || length > BanAppealMaxLength {
		templateData["AppealDraft"] = appealText
		return templateData.AddAlerts(web.ErrorAlert("Your appeal must be between ", BanAppealMinLength, " and ", BanAppealMaxLength, " characters long.")), nil
	}

	config, err := FetchConfig(guild.ID)
	if err != nil {
		return templateData, err
	}

	appeal, err := FileBanAppeal(ctx, config, guild.ID, web.ContextUser(ctx).ID, BanAppealSourceWeb, appealText)
	switch {
	case err == ErrAlreadyAppealed || err == ErrNotBanned || err == ErrBanAppealsDisabled:
		return templateData.AddAlerts(web.ErrorAlert(err.Error())), nil
	case err != nil:
		return templateData, err
	}

	if err := scheduleBanAppealPost(guild.ID, appeal.ID); err != nil {
		if _, deleteErr := appeal.DeleteG(ctx); deleteErr != nil {
			web.CtxLogger(ctx).WithError(deleteErr).Error("failed withdrawing unscheduled ban appeal")
		}
		return templateData, err
	}

	templateData["AppealState"] = banAppealPageSubmitted
	return templateData, nil
}
