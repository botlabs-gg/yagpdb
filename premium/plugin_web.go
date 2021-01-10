package premium

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
)

type CtxKey int

var CtxKeyIsPremium CtxKey = 1

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../premium/assets/premium.html", "templates/plugins/premium.html")

	web.CPMux.Use(PremiumGuildMW)
	web.ServerPublicMux.Use(PremiumGuildMW)

	submux := goji.SubMux()
	web.RootMux.Handle(pat.New("/premium"), submux)
	web.RootMux.Handle(pat.New("/premium/*"), submux)

	submux.Use(web.RequireSessionMiddleware)

	mainHandler := web.ControllerHandler(HandleGetPremiumMainPage, "premium_user_setup")

	submux.Handle(pat.Get("/"), mainHandler)
	submux.Handle(pat.Get(""), mainHandler)

	limiter := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	limiter.SetIPLookups([]string{"CF-Connecting-IP", "X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	submux.Handle(pat.Post("/lookupcode"), tollbooth.LimitHandler(limiter, web.ControllerPostHandler(HandlePostLookupCode, mainHandler, nil)))
	submux.Handle(pat.Post("/redeemcode"), tollbooth.LimitHandler(limiter, web.ControllerPostHandler(HandlePostRedeemCode, mainHandler, nil)))
	submux.Handle(pat.Post("/updateslot/:slotID"), web.ControllerPostHandler(HandlePostUpdateSlot, mainHandler, UpdateData{}))
}

// Add in a template var wether the guild is premium or not
func PremiumGuildMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		guild, tmpl := web.GetBaseCPContextData(r.Context())

		isPremium, err := IsGuildPremium(guild.ID)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).Error("Failed checking if guild is premium")
		}

		tmpl["IsGuildPremium"] = isPremium

		inner.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), CtxKeyIsPremium, isPremium)))
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

	err = DetachSlotFromGuild(r.Context(), parsedSlotID, user.ID)
	if err != nil {
		return tmpl, err
	}

	if data.GuildID != 0 {
		err = AttachSlotToGuild(r.Context(), parsedSlotID, user.ID, data.GuildID)
		if err == ErrGuildAlreadyPremium {
			tmpl.AddAlerts(web.ErrorAlert("Server already has premium from another slot (possibly from another user)"))
		}
	}

	return tmpl, err
}

func ContextPremium(ctx context.Context) bool {
	if v := ctx.Value(CtxKeyIsPremium); v != nil {
		return v.(bool)
	}

	return false
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)
var _ web.ServerHomeWidgetWithOrder = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Premium Status"

	footer := "<p><a href=\"/premium\">Manage your premium slots</a></p>"

	if ContextPremium(r.Context()) {
		username := ""
		discrim := ""

		premiumBy, err := PremiumProvidedBy(ag.ID)
		if err != nil {
			return templateData, err
		}

		user, err := common.BotSession.User(premiumBy)
		if err == nil {
			username = user.Username
			discrim = user.Discriminator
		}

		templateData["WidgetBody"] = template.HTML(fmt.Sprintf("<p>Premium active and provided by <code>%s#%s (%d)</p></code>\n\n%s", html.EscapeString(username), html.EscapeString(discrim), premiumBy, footer))
		templateData["WidgetEnabled"] = true

		return templateData, err
	} else {
		templateData["WidgetDisabled"] = true
		templateData["WidgetBody"] = template.HTML(fmt.Sprintf("<p>Premium not active on this server :(</p>\n\n%s", footer))
	}

	return templateData, nil
}

func (p *Plugin) ServerHomeWidgetOrder() int {
	return 10
}
