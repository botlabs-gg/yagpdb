package serverpreview

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/serverpreview.html
var PageHTML string

//go:embed assets/serverpreview_public.html
var PublicPageHTML string

var panelLogKeyUpdated = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "server_preview_updated", FormatString: "Updated server preview settings"})

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("serverpreview/assets/serverpreview.html", PageHTML)
	web.AddHTMLTemplate("serverpreview/assets/serverpreview_public.html", PublicPageHTML)

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name:     "Server Preview",
		URL:      "server-preview",
		Icon:     "fas fa-id-card",
		NewUntil: time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC),
	})

	mux := goji.SubMux()
	web.CPMux.Handle(pat.New("/server-preview"), mux)
	web.CPMux.Handle(pat.New("/server-preview/*"), mux)
	mux.Use(web.RequireBotMemberMW)
	mux.Use(premium.PremiumGuildMW)

	getHandler := web.ControllerHandler(handleGetSettings, "cp_server_preview")
	postHandler := web.ControllerPostHandler(handlePostSettings, getHandler, PostForm{})
	mux.Handle(pat.Get(""), getHandler)
	mux.Handle(pat.Get("/"), getHandler)
	mux.Handle(pat.Post(""), postHandler)
	mux.Handle(pat.Post("/"), postHandler)
	mux.Handle(pat.Get("/render"), web.APIHandler(handleRenderPreview))

	web.RootMux.HandleFunc(pat.Get("/s/:key"), handlePublicPage)
	web.RootMux.HandleFunc(pat.Get("/s/:key/embed.json"), handlePublicEmbed)
}

type PostForm struct {
	Enabled         bool
	InviteChannel   int64 `valid:"channel,true"`
	ShowMemberCount bool
	ShowDescription bool
	ShowBanner      bool

	Slug        string `valid:",0,32"`
	AccentColor string `valid:",0,7"`
	HideFooter  bool
	CustomText  string `valid:",0,500"`
	Buttons     []Button
}

func handleGetSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	guild, tmpl := web.GetBaseCPContextData(r.Context())

	settings, err := GetSettings(guild.ID)
	if err != nil {
		return tmpl, err
	}

	buttons := ParseButtons(settings.Buttons)
	for len(buttons) < MaxButtons {
		buttons = append(buttons, Button{})
	}

	tmpl["PreviewSettings"] = settings
	tmpl["PreviewButtons"] = buttons
	accent := settings.AccentColor
	if accent == 0 {
		accent = defaultAccentColor
	}
	tmpl["PreviewAccentColor"] = fmt.Sprintf("#%06x", accent)
	version := VersionQuery(settings)
	tmpl["PreviewURL"] = fmt.Sprintf("%s/s/%d%s", web.BaseURL(), guild.ID, version)
	if settings.Slug.Valid && premium.ContextPremium(r.Context()) {
		tmpl["PreviewSlugURL"] = web.BaseURL() + "/s/" + settings.Slug.String + version
	}
	return tmpl, nil
}

func handlePostSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	guild, tmpl := web.GetBaseCPContextData(ctx)
	form := ctx.Value(common.ContextKeyParsedForm).(*PostForm)
	isPremium := premium.ContextPremium(ctx)

	current, err := GetSettings(guild.ID)
	if err != nil {
		return tmpl, err
	}
	if current.Suspended {
		return tmpl.AddAlerts(web.ErrorAlert("Server previews were disabled for this server by YAGPDB staff.")), nil
	}

	if form.Enabled {
		if form.InviteChannel == 0 {
			return tmpl.AddAlerts(web.ErrorAlert("Pick an invite channel, the preview needs an invite for its Join button.")), nil
		}

		perms, err := botrest.GetChannelPermissions(guild.ID, form.InviteChannel)
		if err != nil {
			return tmpl, err
		}
		if perms&discordgo.PermissionCreateInstantInvite == 0 {
			return tmpl.AddAlerts(web.ErrorAlert("YAGPDB is missing the Create Invite permission in the invite channel, give it that permission or pick another channel.")), nil
		}
	}

	updated := *current
	updated.Enabled = form.Enabled
	updated.HideMemberCount = !form.ShowMemberCount
	updated.HideDescription = !form.ShowDescription
	updated.HideBanner = !form.ShowBanner

	if form.InviteChannel != updated.InviteChannelID || (updated.InviteCode == "" && form.InviteChannel != 0) {
		code, err := CreateInvite(form.InviteChannel)
		if err != nil {
			return tmpl.AddAlerts(web.ErrorAlert("Couldn't create an invite in that channel, make sure YAGPDB has the Create Invite permission there.")), nil
		}
		updated.InviteChannelID = form.InviteChannel
		updated.InviteCode = code
	}

	if isPremium {
		if errMsg := applyPremiumSettings(&updated, form); errMsg != "" {
			return tmpl.AddAlerts(web.ErrorAlert(errMsg)), nil
		}
	}

	updated.UpdatedAt = time.Now()
	err = updated.UpsertG(ctx, true, []string{"guild_id"}, boil.Blacklist("guild_id", "suspended"), boil.Infer())
	if err != nil {
		return tmpl, err
	}

	evictSettings(guild.ID, current.Slug, updated.Slug)
	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyUpdated))
	return tmpl, nil
}

// applyPremiumSettings returns a message for the user when the form is invalid.
func applyPremiumSettings(settings *models.ServerPreview, form *PostForm) string {
	slug := strings.ToLower(strings.TrimSpace(form.Slug))
	settings.Slug = null.String{}
	if slug != "" {
		if err := ValidateSlug(slug); err != nil {
			return "Invalid slug: " + err.Error()
		}

		taken, err := models.ServerPreviews(qm.Where("slug = ? AND guild_id != ?", slug, settings.GuildID)).ExistsG(context.Background())
		if err != nil || taken {
			return "That slug is already taken."
		}
		settings.Slug = null.StringFrom(slug)
	}

	settings.AccentColor = 0
	if color := strings.TrimPrefix(strings.TrimSpace(form.AccentColor), "#"); color != "" {
		parsed, err := strconv.ParseInt(color, 16, 32)
		if err != nil || parsed < 0 || parsed > 0xFFFFFF {
			return "Invalid accent colour."
		}
		settings.AccentColor = int(parsed)
	}

	customText := strings.TrimSpace(form.CustomText)
	if err := ValidateCustomText(customText, settings.GuildID); err != nil {
		return "Invalid custom text: " + err.Error()
	}
	settings.CustomText = customText

	var buttons []Button
	for _, b := range form.Buttons {
		b.Label = strings.TrimSpace(b.Label)
		b.URL = strings.TrimSpace(b.URL)
		if b.Label != "" || b.URL != "" {
			buttons = append(buttons, b)
		}
	}
	if err := ValidateButtons(buttons, settings.GuildID); err != nil {
		return "Invalid button: " + err.Error()
	}
	encoded, _ := json.Marshal(buttons)
	if buttons == nil {
		encoded = []byte("[]")
	}
	settings.Buttons = encoded

	settings.HideFooter = form.HideFooter

	if err := ValidatePreviewSize(settings); err != nil {
		return err.Error()
	}
	return ""
}

// handleRenderPreview renders unsaved settings for the dashboard preview. Links aren't checked until saving.
func handleRenderPreview(w http.ResponseWriter, r *http.Request) any {
	ctx := r.Context()
	guild, _ := web.GetBaseCPContextData(ctx)
	isPremium := premium.ContextPremium(ctx)
	query := r.URL.Query()

	current, err := GetSettings(guild.ID)
	if err != nil {
		return err
	}
	settings := *current
	settings.HideMemberCount = query.Get("ShowMemberCount") != "on"
	settings.HideDescription = query.Get("ShowDescription") != "on"
	settings.HideBanner = query.Get("ShowBanner") != "on"

	if isPremium {
		settings.AccentColor = 0
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(query.Get("AccentColor"), "#"), 16, 32); err == nil && parsed >= 0 && parsed <= 0xFFFFFF {
			settings.AccentColor = int(parsed)
		}
		settings.HideFooter = query.Get("HideFooter") == "on"
		settings.CustomText = strings.TrimSpace(query.Get("CustomText"))

		var buttons []Button
		for i := range MaxButtons {
			b := Button{
				Label: strings.TrimSpace(query.Get(fmt.Sprintf("Buttons.%d.Label", i))),
				URL:   strings.TrimSpace(query.Get(fmt.Sprintf("Buttons.%d.URL", i))),
			}
			if b.Label != "" && b.URL != "" {
				buttons = append(buttons, b)
			}
		}
		settings.Buttons, _ = json.Marshal(buttons)
	}

	discordGuild, err := getGuildWithCounts(guild.ID)
	if err != nil {
		return err
	}

	preview := newPreview(&settings, discordGuild)
	if query.Get("InviteChannel") != "" && query.Get("InviteChannel") != "0" {
		preview.InviteURL = "https://discord.gg/"
	}

	if isPremium {
		if err := ValidatePreviewSize(&settings); err != nil {
			return map[string]any{"ok": false, "error": "Can't be saved: " + err.Error()}
		}
	}

	embed := previewEmbed(&settings, preview, isPremium)
	if _, err := embed.Encode(); err != nil {
		return err
	}

	return map[string]any{"ok": true, "component": embed.Component}
}

func handlePublicPage(w http.ResponseWriter, r *http.Request) {
	preview, err := GetPreview(pat.Param(r, "key"))
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("failed loading server preview")
		http.Error(w, "Failed loading this server preview, try again later.", http.StatusInternalServerError)
		return
	}
	isBot := isLinkPreviewBot(r.UserAgent())
	if preview == nil {
		if isBot {
			http.NotFound(w, r)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
		return
	}

	if !isBot {
		http.Redirect(w, r, preview.InviteURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = web.Templates.ExecuteTemplate(w, "serverpreview_public", map[string]any{
		"Preview": preview,
		"BaseURL": template.URL(web.BaseURL()),
		"BotName": botName(),
	})
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("failed rendering server preview page")
	}
}

func handlePublicEmbed(w http.ResponseWriter, r *http.Request) {
	preview, err := GetPreview(pat.Param(r, "key"))
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("failed loading server preview")
		http.Error(w, "failed loading server preview", http.StatusInternalServerError)
		return
	}
	if preview == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.discord.component-embed+json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write(preview.EmbedJSON)
}

var linkPreviewBots = []string{"discordbot", "twitterbot", "slackbot", "facebookexternalhit", "telegrambot", "whatsapp", "linkedinbot", "redditbot", "skypeuripreview", "embedly", "iframely", "applebot", "mastodon", "bluesky"}

// isLinkPreviewBot reports whether the request comes from a crawler building a link preview, which gets the preview page instead of the invite redirect.
func isLinkPreviewBot(userAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	for _, bot := range linkPreviewBots {
		if strings.Contains(userAgent, bot) {
			return true
		}
	}
	return false
}
