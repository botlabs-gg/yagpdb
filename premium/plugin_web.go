package premium

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"goji.io"
	"goji.io/pat"
)

//go:embed assets/premium.html
var PremiumHTML string

//go:embed assets/premium-perks.html
var PremiumPerksHTML string

type CtxKey int

var (
	CtxKeyIsPremium   CtxKey = 1
	CtxKeyPremiumTier CtxKey = 2
)

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("premium/assets/premium.html", PremiumHTML)
	web.AddHTMLTemplate("premium/assets/premium-perks.html", PremiumPerksHTML)

	web.CPMux.Use(PremiumGuildMW)
	web.ServerPublicMux.Use(PremiumGuildMW)

	submux := goji.SubMux()
	web.RootMux.Handle(pat.New("/premium"), submux)
	web.RootMux.Handle(pat.New("/premium/*"), submux)
	web.RootMux.Handle(pat.New("/premium-perks"), web.RenderHandler(nil, "premium_perks"))
	web.RootMux.Handle(pat.New("/premium-perks/*"), web.ControllerHandler(nil, "premium_perks"))

	submux.Use(web.RequireSessionMiddleware)

	mainHandler := web.ControllerHandler(HandleGetPremiumMainPage, "premium_user_setup")

	submux.Handle(pat.Get("/"), mainHandler)
	submux.Handle(pat.Get(""), mainHandler)

	limiter := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	limiter.SetIPLookups([]string{"CF-Connecting-IP", "X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	submux.Handle(pat.Post("/lookupcode"), tollbooth.LimitHandler(limiter, web.ControllerPostHandler(HandlePostLookupCode, mainHandler, nil)))
	submux.Handle(pat.Post("/redeemcode"), tollbooth.LimitHandler(limiter, web.ControllerPostHandler(HandlePostRedeemCode, mainHandler, nil)))
	submux.Handle(pat.Post("/updateslot/:slotID"), web.ControllerPostHandler(HandlePostUpdateSlot, mainHandler, UpdateData{}))

	web.CPMux.Handle(pat.Post("/premium/detach"), web.ControllerPostHandler(HandlePostDetachGuildSlot, web.RenderHandler(nil, "cp_premium_detach"), nil))
}

// PremiumGuildMW adds premium data to context and tmpl vars
func PremiumGuildMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		guild, tmpl := web.GetBaseCPContextData(r.Context())

		isPremium, err := IsGuildPremium(guild.ID)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).Error("Failed checking if guild is premium")
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxKeyIsPremium, isPremium)
		tmpl["IsGuildPremium"] = isPremium

		if isPremium {

			tier, err := GuildPremiumTier(guild.ID)
			if err != nil {
				web.CtxLogger(ctx).WithError(err).Error("Failed retrieving guild premium tier")
			}

			tmpl["GuildPremiumTier"] = tier
			ctx = context.WithValue(ctx, CtxKeyPremiumTier, tier)
		}

		inner.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(mw)
}

func HandleGetPremiumMainPage(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, tmpl = web.GetCreateTemplateData(r.Context())

	user := web.ContextUser(r.Context())
	slots, err := UserPremiumSlots(r.Context(), user.ID)
	if err != nil {
		return tmpl, err
	}

	guilds, _ := web.GetUserGuilds(r.Context())

	tmpl["UserGuilds"] = guilds
	tmpl["PremiumSlotDurationRemaining"] = SlotDurationLeft
	tmpl["PremiumSlots"] = slots
	return tmpl, nil
}

func HandlePostLookupCode(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, tmpl = web.GetCreateTemplateData(r.Context())

	code := r.FormValue("code")
	if code == "" {
		return tmpl.AddAlerts(web.ErrorAlert("No code provided")), nil
	}

	codeModel, err := LookupCode(r.Context(), code)
	if err != nil {
		if err == ErrCodeNotFound {
			return tmpl.AddAlerts(web.ErrorAlert("Code not found")), nil
		}

		return tmpl, err
	}

	if codeModel.UserID.Valid {
		return tmpl.AddAlerts(web.ErrorAlert("That code is already redeemed")), nil
	}

	tmpl["QueriedCode"] = codeModel
	return tmpl, nil
}

func HandlePostRedeemCode(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, tmpl = web.GetCreateTemplateData(r.Context())
	user := web.ContextUser(r.Context())

	code := r.FormValue("code")
	if code == "" {
		return tmpl.AddAlerts(web.ErrorAlert("No code provided")), nil
	}

	err = RedeemCode(r.Context(), code, user.ID)
	return tmpl, err
}

type UpdateData struct {
	GuildID int64
}

func HandlePostUpdateSlot(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, tmpl = web.GetCreateTemplateData(r.Context())
	data := r.Context().Value(common.ContextKeyParsedForm).(*UpdateData)
	user := web.ContextUser(r.Context())

	strSlotID := pat.Param(r, "slotID")
	parsedSlotID, _ := strconv.ParseInt(strSlotID, 10, 64)
	slot, err := models.PremiumSlots(qm.Where("id = ?", parsedSlotID), qm.Select(models.PremiumSlotColumns.GuildID)).One(r.Context(), common.PQ)
	if err != nil {
		return tmpl, errors.WithMessage(err, "Failed retrieving slot")
	}

	if slot.GuildID.Int64 == data.GuildID {
		return tmpl, nil
	}

	err = DetachSlotFromGuild(r.Context(), common.PQ, parsedSlotID, user.ID)
	if err != nil {
		return tmpl, err
	}

	if data.GuildID != 0 {
		err = AttachSlotToGuild(r.Context(), common.PQ, parsedSlotID, user.ID, data.GuildID)
		if err == ErrGuildAlreadyPremium {
			tmpl.AddAlerts(web.ErrorAlert("Server already has premium from another slot (possibly from another user)"))
			return tmpl, err
		}
	}

	if err != nil {
		return tmpl, err
	}

	return tmpl, err
}

func ContextPremium(ctx context.Context) bool {
	if confAllGuildsPremium.GetBool() {
		return true
	}
	if v := ctx.Value(CtxKeyIsPremium); v != nil {
		return v.(bool)
	}

	return false
}

func ContextPremiumTier(ctx context.Context) PremiumTier {
	if confAllGuildsPremium.GetBool() {
		return PremiumTierPremium
	}

	if v := ctx.Value(CtxKeyPremiumTier); v != nil {
		return v.(PremiumTier)
	}

	return PremiumTierNone
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)
var _ web.ServerHomeWidgetWithOrder = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Premium Status"

	footer := "<p><a href=\"/premium\">Manage your user premium slots</a></p>"

	if ContextPremium(r.Context()) {
		body := strings.Builder{}

		for _, v := range GuildPremiumSources {
			tier, status, err := v.GuildPremiumDetails(ag.ID)
			if err != nil {
				return nil, err
			}

			if tier <= PremiumTierNone {
				continue
			}

			if _, ok := v.(*SlotGuildPremiumSource); ok {
				// special handling for this since i was a bit lazy
				username := ""

				premiumBy, err := PremiumProvidedBy(ag.ID)
				if err != nil {
					return templateData, err
				}

				user, err := common.BotSession.User(premiumBy)
				if err == nil {
					username = user.String()
				}

				detForm := fmt.Sprintf(`<form data-async-form action="/manage/%d/premium/detach">
			<button type="submit" class="btn btn-danger">Detach premium slot</button>
		</form>`, ag.ID)

				body.WriteString(fmt.Sprintf("<p>Premium tier <b>%s</b> active and provided by user <code>%s (%d)</p></code>\n\n%s", tier.String(), html.EscapeString(username), premiumBy, detForm))
			} else {
				body.WriteString(fmt.Sprintf("<p class=\"mt-3\">Premium tier <b>%s</b> active and provided by %s: %s</p>", tier.String(), v.Name(), status))
			}
		}

		templateData["WidgetEnabled"] = true
		templateData["WidgetBody"] = template.HTML(body.String() + footer)

		return templateData, nil
	} else {
		templateData["WidgetDisabled"] = true
		templateData["WidgetBody"] = template.HTML(fmt.Sprintf("<p>Premium not active on this server :(</p>\n\n%s", footer))
	}

	return templateData, nil
}

func (p *Plugin) ServerHomeWidgetOrder() int {
	return 10
}

func HandlePostDetachGuildSlot(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	slot, err := models.PremiumSlots(qm.Where("guild_id = ?", activeGuild.ID)).OneG(r.Context())
	if err != nil {
		if err == sql.ErrNoRows {
			templateData.AddAlerts(web.ErrorAlert("No premium slot attached to this server"))
			return templateData, nil
		}

		return templateData, err
	}

	err = DetachSlotFromGuild(r.Context(), common.PQ, slot.ID, slot.UserID)

	return templateData, err
}
