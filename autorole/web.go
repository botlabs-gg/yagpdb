package autorole

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
)

type Form struct {
	GeneralConfig `valid:"traverse"`
}

var _ web.SimpleConfigSaver = (*Form)(nil)

func (f Form) Save(guildID int64) error {
	pubsub.Publish("autorole_stop_processing", guildID, nil)

	err := common.SetRedisJson(KeyGeneral(guildID), f.GeneralConfig)
	if err != nil {
		return err
	}

	return nil
}

func (f Form) Name() string {
	return "Autorole"
}

func (p *Plugin) InitWeb() {
	tmplPathSettings := "templates/plugins/autorole.html"
	if common.Testing {
		tmplPathSettings = "../../autorole/assets/autorole.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings))

	muxer := goji.SubMux()

	web.CPMux.Handle(pat.New("/autorole"), muxer)
	web.CPMux.Handle(pat.New("/autorole/*"), muxer)

	muxer.Use(web.RequireFullGuildMW) // need roles
	muxer.Use(web.RequireBotMemberMW) // need the bot's role
	muxer.Use(web.RequirePermMW(discordgo.PermissionManageRoles))

	getHandler := web.RenderHandler(handleGetAutoroleMainPage, "cp_autorole")

	muxer.Handle(pat.Get(""), getHandler)
	muxer.Handle(pat.Get("/"), getHandler)

	muxer.Handle(pat.Post("/fullscan"), web.ControllerPostHandler(handlePostFullScan, getHandler, nil, "Triggered a full autorole scan"))

	muxer.Handle(pat.Post(""), web.SimpleConfigSaverHandler(Form{}, getHandler))
	muxer.Handle(pat.Post("/"), web.SimpleConfigSaverHandler(Form{}, getHandler))
}

func handleGetAutoroleMainPage(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	general, err := GetGeneralConfig(activeGuild.ID)
	web.CheckErr(tmpl, err, "Failed retrieving general config (contact support)", web.CtxLogger(r.Context()).Error)
	tmpl["Autorole"] = general

	var proc int
	common.RedisPool.Do(radix.Cmd(&proc, "GET", KeyProcessing(activeGuild.ID)))
	tmpl["Processing"] = proc
	tmpl["ProcessingETA"] = int(proc / 60)

	fullScanActive, err := WorkingOnFullScan(activeGuild.ID)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("failed checking full scan")
	}
	tmpl["FullScanActive"] = fullScanActive

	return tmpl

}

func handlePostFullScan(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	err := botRestPostFullScan(activeGuild.ID)
	if err != nil {
		if err == ErrAlreadyProcessingFullGuild {
			return tmpl.AddAlerts(web.ErrorAlert("Already processing, please wait.")), nil
		}

		return tmpl, errors.WithMessage(err, "botrest")
	}

	return tmpl, nil
}
