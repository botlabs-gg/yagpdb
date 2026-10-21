package moderation

import (
	_ "embed"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/botlabs-gg/yagpdb/v2/web/discorddata"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"goji.io/pat"
)

//go:embed assets/moderation_ban_appeal.html
var banAppealPageHTML string

type banAppealPageState string

const (
	banAppealPageInvalid   banAppealPageState = "invalid"
	banAppealPageDisabled  banAppealPageState = "disabled"
	banAppealPagePending   banAppealPageState = "pending"
	banAppealPageRejected  banAppealPageState = "rejected"
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

	web.RootMux.Handle(pat.Get("/:guild/appeal"), getHandler)
	web.RootMux.Handle(pat.Post("/:guild/appeal"), tollbooth.LimitHandler(submitLimiter, postHandler))
}

// bannedAppealGuild is nil unless the user is banned there, so a link reveals nothing about other servers.
func bannedAppealGuild(r *http.Request, templateData web.TemplateData) (*dstate.GuildSet, *discordgo.GuildBan) {
	guildID, err := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	if err != nil {
		return nil, nil
	}

	guild, err := discorddata.GetFullGuild(guildID)
	if err != nil || guild == nil {
		return nil, nil
	}

	ban, err := CurrentBan(guild.ID, web.ContextUser(r.Context()).ID)
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).WithField("guild", guild.ID).Warn("failed checking ban for appeal page")
		return nil, nil
	}
	if ban == nil {
		return nil, nil
	}

	templateData["GuildName"] = guild.Name
	templateData["GuildIcon"] = guild.IconURL("128")
	return guild, ban
}

func handleGetBanAppealPage(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, templateData := web.GetCreateTemplateData(ctx)
	templateData["BanAppealMinLength"] = BanAppealMinLength
	templateData["BanAppealMaxLength"] = BanAppealMaxLength

	if _, alreadyDecided := templateData["AppealState"]; alreadyDecided {
		return templateData, nil
	}

	guild, ban := bannedAppealGuild(r, templateData)
	if guild == nil {
		templateData["AppealState"] = banAppealPageInvalid
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

	open, err := FindOpenBanAppeal(ctx, guild.ID, web.ContextUser(ctx).ID)
	if err != nil {
		return templateData, err
	}
	if open != nil && open.RejectedAt.Valid {
		templateData["AppealState"] = banAppealPageRejected
		return templateData, nil
	}
	if open != nil {
		templateData["AppealState"] = banAppealPagePending
		templateData["AppealedAt"] = open.CreatedAt
		return templateData, nil
	}

	templateData["AppealState"] = banAppealPageForm
	templateData["BanReason"] = BanReasonForBannedUser(ban.Reason)
	return templateData, nil
}

func handlePostBanAppeal(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, templateData := web.GetCreateTemplateData(ctx)

	guild, _ := bannedAppealGuild(r, templateData)
	if guild == nil {
		templateData["AppealState"] = banAppealPageInvalid
		return templateData, nil
	}

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
	case err == ErrNotBanned:
		templateData["AppealState"] = banAppealPageInvalid
		return templateData, nil
	case err == ErrAlreadyAppealed || err == ErrBanAppealsDisabled:
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
