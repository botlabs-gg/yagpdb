package automod_legacy

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
)

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

type GeneralForm struct {
	Enabled bool
}

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../automod_legacy/assets/automod_legacy.html", "templates/plugins/automod_legacy.html")

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Basic Automoderator",
		URL:  "automod_legacy",
		Icon: "fas fa-robot",
	})

	autmodMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/automod_legacy/*"), autmodMux)
	web.CPMux.Handle(pat.New("/automod_legacy"), autmodMux)

	// Alll handlers here require guild channels present
	autmodMux.Use(web.RequireGuildChannelsMiddleware)
	autmodMux.Use(web.RequireBotMemberMW)
	autmodMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageMessages))

	getHandler := web.RenderHandler(HandleAutomod, "cp_automod_legacy")

	autmodMux.Handle(pat.Get("/"), getHandler)
	autmodMux.Handle(pat.Get(""), getHandler)

	// Post handlers
	autmodMux.Handle(pat.Post("/"), ExtraPostMW(web.SimpleConfigSaverHandler(Config{}, getHandler)))
	autmodMux.Handle(pat.Post(""), ExtraPostMW(web.SimpleConfigSaverHandler(Config{}, getHandler)))
}

func HandleAutomod(w http.ResponseWriter, r *http.Request) interface{} {
	g, templateData := web.GetBaseCPContextData(r.Context())

	config, err := GetConfig(g.ID)
	web.CheckErr(templateData, err, "Failed retrieving rules", web.CtxLogger(r.Context()).Error)

	templateData["AutomodConfig"] = config
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(g.ID) + "/automod_legacy/"

	return templateData
}

// Invalidates the cache
func ExtraPostMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		activeGuild, _ := web.GetBaseCPContextData(r.Context())
		pubsub.Publish("update_automod_legacy_rules", activeGuild.ID, nil)
		featureflags.MarkGuildDirty(activeGuild.ID)
		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Basic Automod"
	templateData["SettingsPath"] = "/automod_legacy"

	config, err := GetConfig(g.ID)
	if err != nil {
		return templateData, err
	}

	if config.Enabled {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<ul>
	<li>Slowmode: %s</li>
	<li>Mass mention: %s</li>
	<li>Server invites: %s</li>
	<li>Any links: %s</li>
	<li>Banned words: %s</li>
	<li>Banned websites: %s</li>
</ul>`

	slowmode := web.EnabledDisabledSpanStatus(config.Spam.Enabled)
	massMention := web.EnabledDisabledSpanStatus(config.Mention.Enabled)
	invites := web.EnabledDisabledSpanStatus(config.Invite.Enabled)
	links := web.EnabledDisabledSpanStatus(config.Links.Enabled)
	words := web.EnabledDisabledSpanStatus(config.Words.Enabled)
	sites := web.EnabledDisabledSpanStatus(config.Sites.Enabled)

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, slowmode,
		massMention, invites, links, words, sites))

	return templateData, nil
}
