package twitch

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/twitch/models"
	"github.com/botlabs-gg/yagpdb/v2/web"

	"github.com/nicklaw5/helix/v2"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/twitch.html
var PageHTML string

var (
	panelLogKeyAddedFeed    = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitch_added_feed", FormatString: "Added twitch feed from %s"})
	panelLogKeyAnnouncement = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitch_announcement", FormatString: "Updated Twitch Announcement"})
	panelLogKeyRemovedFeed  = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitch_removed_feed", FormatString: "Removed twitch feed from %s"})
	panelLogKeyUpdatedFeed  = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitch_updated_feed", FormatString: "Updated twitch feed from %s"})
)

type TwitchFeedForm struct {
	TwitchUsername  string
	DiscordChannel  int64 `valid:"channel,false"`
	MentionEveryone bool
	MentionRoles    []int64
	PublishVOD      bool
	Enabled         bool
}

type TwitchAnnouncementForm struct {
	Message string `json:"message" valid:"template,5000"`
	Enabled bool
}

type ContextKey int

const (
	ContextKeySub ContextKey = iota
)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("twitch/assets/twitch.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "Twitch",
		URL:  "twitch",
		Icon: "fab fa-twitch",
	})

	mux := goji.SubMux()
	web.CPMux.Handle(pat.New("/twitch/*"), mux)
	web.CPMux.Handle(pat.New("/twitch"), mux)

	// All handlers here require guild channels present
	mux.Use(web.RequireBotMemberMW)
	mux.Use(web.RequirePermMW(discordgo.PermissionMentionEveryone))
	mux.Use(premium.PremiumGuildMW)

	mainGetHandler := web.ControllerHandler(p.HandleTwitch, "cp_twitch")

	mux.Handle(pat.Get("/"), mainGetHandler)
	mux.Handle(pat.Get(""), mainGetHandler)

	addHandler := web.ControllerPostHandler(p.HandleNew, mainGetHandler, TwitchFeedForm{})

	mux.Handle(pat.Post(""), addHandler)
	mux.Handle(pat.Post("/"), addHandler)
	mux.Handle(pat.Post("/announcement"), web.ControllerPostHandler(p.HandleTwitchAnnouncement, mainGetHandler, TwitchAnnouncementForm{}))
	mux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(p.HandleEdit), mainGetHandler, TwitchFeedForm{}))
	mux.Handle(pat.Post("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
	mux.Handle(pat.Get("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
}

func (p *Plugin) HandleTwitch(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	subs, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)), qm.OrderBy("id DESC")).AllG(ctx)
	if err != nil {
		return templateData, err
	}

	templateData["TwitchSubs"] = subs

	announcement, err := models.FindTwitchAnnouncementG(ctx, activeGuild.ID)
	if err != nil {
		announcement = &models.TwitchAnnouncement{
			GuildID: activeGuild.ID,
			Message: `{{if not .IsLive}} 
{{.User}} went offline! Catch the VOD here:
{{.VODUrl}}
{{else}}
{{.User}} is now live!
{{.URL}}
{{end}}`,
			Enabled: false,
		}
	}
	templateData["TwitchAnnouncement"] = announcement

	return templateData, nil
}

func (p *Plugin) HandleTwitchAnnouncement(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	// Check premium for custom announcements
	if !premium.ContextPremium(ctx) {
		return templateData.AddAlerts(web.ErrorAlert("Custom announcements are premium only")), nil
	}

	form := ctx.Value(common.ContextKeyParsedForm).(*TwitchAnnouncementForm)

	announcement := &models.TwitchAnnouncement{
		GuildID: activeGuild.ID,
		Message: form.Message,
		Enabled: form.Enabled,
	}

	err := announcement.UpsertG(ctx, true, []string{"guild_id"}, boil.Whitelist("message", "enabled"), boil.Infer())
	if err != nil {
		return templateData, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAnnouncement))

	return templateData, nil
}

func (p *Plugin) HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	// Check limits
	maxFeeds := MaxFeedsForContext(ctx)

	count, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID))).CountG(ctx)
	if err != nil {
		return templateData, err
	}

	if int(count) >= maxFeeds {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max %d feeds allowed (upgrade to premium for more)", maxFeeds))), nil
	}

	data := ctx.Value(common.ContextKeyParsedForm).(*TwitchFeedForm)

	// Extract username from URL if a URL was provided
	username := extractTwitchUsername(data.TwitchUsername)

	// Validate Twitch User
	// We need to get the user ID from Twitch
	users, err := p.HelixClient.GetUsers(&helix.UsersParams{
		Logins: []string{username},
	})
	if err != nil {
		logger.Error("Failed to get twitch user", "err", err)
		return templateData.AddAlerts(web.ErrorAlert("Failed to get twitch user")), nil
	}

	if len(users.Data.Users) == 0 {
		return templateData.AddAlerts(web.ErrorAlert("Twitch user not found")), nil
	}

	twitchUser := users.Data.Users[0]

	sub := &models.TwitchChannelSubscription{
		GuildID:         discordgo.StrID(activeGuild.ID),
		ChannelID:       discordgo.StrID(data.DiscordChannel),
		TwitchUserID:    twitchUser.ID,
		TwitchUsername:  twitchUser.Login,
		MentionEveryone: data.MentionEveryone,
		MentionRoles:    data.MentionRoles,
		PublishVod:      data.PublishVOD,
	}

	err = sub.InsertG(ctx, boil.Infer())
	if err != nil {
		return templateData, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAddedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.TwitchUsername}))

	return templateData, nil
}

// extractTwitchUsername extracts the username from a Twitch URL or returns the input if it's already a username
// Supports formats like:
// - https://twitch.tv/username
// - https://www.twitch.tv/username
// - twitch.tv/username
// - username
func extractTwitchUsername(input string) string {
	input = strings.TrimSpace(input)

	// If it doesn't contain a slash, assume it's already a username
	if !strings.Contains(input, "/") {
		return input
	}

	// Parse as URL
	// Add scheme if missing
	urlStr := input
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		urlStr = "https://" + input
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, return the original input
		return input
	}

	// Check if it's a Twitch domain
	if parsedURL.Host != "twitch.tv" && parsedURL.Host != "www.twitch.tv" {
		return input
	}

	// Extract the first path segment as the username
	path := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return input
}

func BaseEditHandler(inner web.ControllerHandlerFunc) web.ControllerHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
		ctx := r.Context()
		activeGuild, templateData := web.GetBaseCPContextData(ctx)

		id, err := strconv.Atoi(pat.Param(r, "item"))
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Invalid feed ID")), err
		}

		sub, err := models.FindTwitchChannelSubscriptionG(ctx, id)
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Failed retrieving that feed item")), err
		}

		if sub.GuildID != discordgo.StrID(activeGuild.ID) {
			return templateData.AddAlerts(web.ErrorAlert("This appears to belong somewhere else...")), nil
		}

		ctx = context.WithValue(ctx, ContextKeySub, sub)

		return inner(w, r.WithContext(ctx))
	}
}

func (p *Plugin) HandleEdit(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	data := ctx.Value(common.ContextKeyParsedForm).(*TwitchFeedForm)
	sub := ctx.Value(ContextKeySub).(*models.TwitchChannelSubscription)

	// Check if we're trying to enable a disabled feed
	if !sub.Enabled && data.Enabled {
		// Count currently enabled feeds
		enabledCount, err := models.TwitchChannelSubscriptions(
			models.TwitchChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)),
			models.TwitchChannelSubscriptionWhere.Enabled.EQ(true),
		).CountG(ctx)
		if err != nil {
			return templateData, err
		}

		maxFeeds := MaxFeedsForContext(ctx)
		if int(enabledCount) >= maxFeeds {
			return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max %d enabled feeds allowed (%d for premium servers)", GuildMaxEnabledFeeds, GuildMaxEnabledFeedsPremium))), nil
		}
	}

	sub.MentionEveryone = data.MentionEveryone
	sub.MentionRoles = data.MentionRoles
	sub.PublishVod = data.PublishVOD
	sub.Enabled = data.Enabled
	sub.ChannelID = discordgo.StrID(data.DiscordChannel)

	_, err := sub.UpdateG(ctx, boil.Infer())
	if err != nil {
		return templateData, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.TwitchUsername}))

	return templateData, nil
}

func (p *Plugin) HandleRemove(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	sub := ctx.Value(ContextKeySub).(*models.TwitchChannelSubscription)

	_, err = sub.DeleteG(ctx)
	if err != nil {
		return
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyRemovedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.TwitchUsername}))
	return
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Twitch feeds"
	templateData["SettingsPath"] = "/twitch"

	numFeeds, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)), models.TwitchChannelSubscriptionWhere.Enabled.EQ(true)).CountG(r.Context())
	if err != nil {
		return templateData, err
	}

	if numFeeds > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<p>Active Twitch feeds: <code>%d</code></p>`
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, numFeeds))

	return templateData, nil
}
