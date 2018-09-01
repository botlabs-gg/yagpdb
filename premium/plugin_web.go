package premium

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
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

	submux.Handle(pat.Get("/"), web.ControllerHandler(HandleGetPremiumMainPage, "premium_user_setup"))
	submux.Handle(pat.Get(""), web.ControllerHandler(HandleGetPremiumMainPage, "premium_user_setup"))
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
	slots, err := UserPremiumSlots(user.ID)
	if err != nil {
		return tmpl, err
	}

	tmpl["PremiumSlots"] = slots
	return tmpl, nil
}

func HandlePostLookupCode(w http.ResponseWriter, r *http.Request) {

}

func HandlePostRedeemCode(w http.ResponseWriter, r *http.Request) {

}

func HandlePostSetSlotGuild(w http.ResponseWriter, r *http.Request) {

}
