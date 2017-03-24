package reputation

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"gopkg.in/nullbio/null.v6"
	"html/template"
	"net/http"
)

type PostConfigForm struct {
	PointsName             string `valid:",50"`
	Enabled                bool
	Cooldown               int `valid:"0,86401"` // One day
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
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/reputation.html"))

	mainGetHandler := web.RenderHandler(HandleGetReputation, "cp_reputation")

	web.CPMux.Handle(pat.Get("/reputation"), mainGetHandler)
	web.CPMux.Handle(pat.Get("/reputation/"), mainGetHandler)
	web.CPMux.Handle(pat.Post("/reputation"), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}, "Updated reputatoin config"))
	web.CPMux.Handle(pat.Post("/reputation/"), web.ControllerPostHandler(HandlePostReputation, mainGetHandler, PostConfigForm{}, "Updated reputatoin config"))
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
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/reputation"

	form := r.Context().Value(common.ContextKeyParsedForm).(*PostConfigForm)
	conf := form.RepConfig()
	conf.GuildID = common.MustParseInt(activeGuild.ID)

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
