package premium

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strconv"
)

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	tmplPathSettings := "templates/plugins/premium.html"
	if common.Testing {
		tmplPathSettings = "../../premium/assets/premium.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings))

	web.CPMux.Use(CPMiddlware)

	submux := goji.SubMux()
	web.RootMux.Handle(pat.New("/premium"), submux)
	web.RootMux.Handle(pat.New("/premium/*"), submux)

	submux.Use(web.RequireSessionMiddleware)

	mainHandler := web.ControllerHandler(HandleGetPremiumMainPage, "premium_user_setup")

	submux.Handle(pat.Get("/"), mainHandler)
	submux.Handle(pat.Get(""), mainHandler)

	submux.Handle(pat.Post("/lookupcode"), web.ControllerPostHandler(HandlePostLookupCode, mainHandler, nil, ""))
	submux.Handle(pat.Post("/redeemcode"), web.ControllerPostHandler(HandlePostRedeemCode, mainHandler, nil, ""))
	submux.Handle(pat.Post("/updateslot/:slotID"), web.ControllerPostHandler(HandlePostUpdateSlot, mainHandler, UpdateData{}, ""))
}

// Add in a template var wether the guild is premium or not
func CPMiddlware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		guild, tmpl := web.GetBaseCPContextData(r.Context())

		isPremium, err := IsGuildPremium(guild.ID)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).Error("Failed checking if guild is premium")
		}

		tmpl["IsGuildPremium"] = isPremium

		inner.ServeHTTP(w, r)
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

	if data.GuildID != 0 {
		err = AttachSlotToGuild(r.Context(), parsedSlotID, user.ID, data.GuildID)
		if err == ErrGuildAlreadyPremium {
			tmpl.AddAlerts(web.ErrorAlert("Server already has premium from another slot (possibly from another user)"))
		}
	} else {
		err = DetachSlotFromGuild(r.Context(), parsedSlotID, user.ID)
	}

	return tmpl, err
}
