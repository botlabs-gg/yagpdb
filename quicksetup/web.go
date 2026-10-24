package quicksetup

import (
	_ "embed"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"goji.io/pat"
)

//go:embed assets/quicksetup.html
var pageHTML string

type moduleView struct {
	*module
	AlreadySetUp bool
	MissingPerms []string
}

type showcasedFeature struct {
	Name, Description, Path, Icon, Color string
}

type pluginFeature struct {
	pluginSysName string
	showcasedFeature
}

var suggestedFeatures = []pluginFeature{
	{"custom_commands", showcasedFeature{"Custom Commands", "Make your own commands and automatic replies.", "/customcommands", "fas fa-terminal", "primary"}},
	{"role_commands", showcasedFeature{"Role Menus", "Let members pick their own roles, like colours or pings.", "/rolecommands/", "fas fa-tags", "info"}},
	{"reputation", showcasedFeature{"Reputation", "Members thank each other for help and earn points.", "/reputation", "fas fa-star", "warning"}},
	{"server_stats", showcasedFeature{"Server Stats", "See how many people join, leave and chat each day.", "/stats", "fas fa-chart-line", "success"}},
	{"messagecreator", showcasedFeature{"Message Creator", "Post good looking announcements and embeds as the bot.", "/messagecreator", "fas fa-pen-fancy", "danger"}},
	{"youtube", showcasedFeature{"YouTube", "Announce new videos from your favourite channels.", "/youtube", "fab fa-youtube", "danger"}},
	{"twitch", showcasedFeature{"Twitch", "Announce when streamers go live.", "/twitch", "fab fa-twitch", "info"}},
	{"reddit", showcasedFeature{"Reddit", "Post new submissions from subreddits.", "/reddit", "fab fa-reddit", "warning"}},
	{"rss", showcasedFeature{"RSS Feeds", "Post updates from any website with an RSS feed.", "/rss", "fas fa-rss", "warning"}},
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("quicksetup/assets/quicksetup.html", pageHTML)

	web.AddSidebarItem(web.SidebarCategoryBelowHome, &web.SidebarItem{
		Name: "Quick Setup",
		URL:  "setup",
		Icon: "fas fa-magic",
		New:  true,
	})

	getHandler := web.RequireBotMemberMW(web.ControllerHandler(handleGetSetup, "cp_quick_setup"))
	postHandler := web.RequireBotMemberMW(web.ControllerPostHandler(handlePostSetup, getHandler, SetupForm{}))

	web.CPMux.Handle(pat.Get("/setup"), skipFinishedSetupFromInvite(getHandler))
	web.CPMux.Handle(pat.Get("/setup/"), skipFinishedSetupFromInvite(getHandler))
	web.CPMux.Handle(pat.Post("/setup"), postHandler)
	web.CPMux.Handle(pat.Post("/setup/skip"), http.HandlerFunc(handlePostSkip))

	web.CPMux.Use(setupPendingBannerMW)
}

func serverHomeURL(guildID int64) string {
	return "/manage/" + strconv.FormatInt(guildID, 10) + "/home"
}

// skipFinishedSetupFromInvite sends someone who just added the bot to a
// server that was set up before straight to its home page. Opening the setup
// from the dashboard always shows it, so it can be run again.
func skipFinishedSetupFromInvite(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("from") == "invite" {
			if guild := web.ContextGuild(r.Context()); guild != nil {
				if state, err := getSetupState(guild.ID); err == nil && state != setupStateNone {
					http.Redirect(w, r, serverHomeURL(guild.ID), http.StatusSeeOther)
					return
				}
			}
		}

		inner.ServeHTTP(w, r)
	})
}

func setupPendingBannerMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		if r.Method == http.MethodGet && strings.HasSuffix(path, "/home") && !web.GetIsReadOnly(r.Context()) {
			if guild := web.ContextGuild(r.Context()); guild != nil {
				if state, err := getSetupState(guild.ID); err == nil && state == setupStateNone {
					r = r.WithContext(web.SetContextTemplateData(r.Context(), map[string]any{"QuickSetupPending": true}))
				}
			}
		}

		inner.ServeHTTP(w, r)
	})
}

func handleGetSetup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	guild, templateData := web.GetBaseCPContextData(ctx)

	if _, applied := templateData["SetupResults"]; applied {
		templateData["SuggestedFeatures"] = loadedFeatures(suggestedFeatures)
		templateData["PremiumPerks"] = premium.AvailablePerks()
		return templateData, nil
	}

	botPerms, _ := ctx.Value(common.ContextKeyBotPermissions).(int64)

	var views []*moduleView
	for _, m := range availableModules() {
		alreadySetUp, err := m.alreadySetUp(ctx, guild.ID)
		if err != nil {
			web.CtxLogger(ctx).WithError(err).WithField("module", m.ID).Error("failed checking if a quick setup module is set up")
		}

		views = append(views, &moduleView{module: m, AlreadySetUp: alreadySetUp, MissingPerms: missingPermNames(botPerms, m.RequiredPerms)})
	}

	templateData["SetupModules"] = views
	templateData["FromInvite"] = r.FormValue("from") == "invite"
	templateData["ReadOnly"] = web.GetIsReadOnly(ctx)
	templateData["DefaultWelcomeMessage"] = DefaultWelcomeMessage
	templateData["InviteURL"] = web.BotInviteURL(guild.ID)

	if systemChannel, err := strconv.ParseInt(guild.SystemChannelID, 10, 64); err == nil {
		templateData["DefaultWelcomeChannel"] = systemChannel
	}

	if config, err := moderation.FetchConfig(guild.ID); err == nil {
		templateData["CurrentModlogChannel"] = config.ActionChannel
	}

	return templateData, nil
}

func handlePostSetup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	guild, templateData := web.GetBaseCPContextData(ctx)

	form := ctx.Value(common.ContextKeyParsedForm).(*SetupForm)
	form.Modules = slices.DeleteFunc(form.Modules, func(id string) bool { return !moduleAvailable(id) })
	if len(form.Modules) == 0 {
		return templateData.AddAlerts(web.ErrorAlert("Pick at least one feature to set up, or skip the setup.")), nil
	}

	templateData["SetupResults"] = applySetup(ctx, form)

	if err := setSetupState(guild.ID, setupStateCompleted); err != nil {
		web.CtxLogger(ctx).WithError(err).Error("failed saving quick setup state")
	}

	return templateData, nil
}

func handlePostSkip(w http.ResponseWriter, r *http.Request) {
	guild := web.ContextGuild(r.Context())

	if err := setSetupState(guild.ID, setupStateSkipped); err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("failed saving quick setup state")
	}

	http.Redirect(w, r, serverHomeURL(guild.ID), http.StatusSeeOther)
}

func loadedFeatures(features []pluginFeature) []showcasedFeature {
	var loaded []showcasedFeature
	for _, feature := range features {
		if common.PluginLoaded(feature.pluginSysName) {
			loaded = append(loaded, feature.showcasedFeature)
		}
	}

	return loaded
}
