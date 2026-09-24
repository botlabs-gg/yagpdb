package rss

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/rss/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/rss.html
var PageHTML string
var (
	panelLogKeyAddedFeed   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "rss_added_feed", FormatString: "Added RSS feed: %s"})
	panelLogKeyUpdatedFeed = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "rss_updated_feed", FormatString: "Updated RSS feed: %s"})
	panelLogKeyRemovedFeed = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "rss_removed_feed", FormatString: "Removed RSS feed: %s"})
)

const (
	GuildMaxRSSFeedsFree    = 2
	GuildMaxRSSFeedsPremium = 10
)

func MaxRSSFeedsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return GuildMaxRSSFeedsPremium
	}
	return GuildMaxRSSFeedsFree
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("rss/assets/rss.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "RSS Feeds",
		URL:  "rss",
		Icon: "fa fa-rss",
	})

	rssMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/rss/*"), rssMux)
	web.CPMux.Handle(pat.New("/rss"), rssMux)

	rssMux.Use(web.RequireBotMemberMW)
	rssMux.Use(web.RequirePermMW(discordgo.PermissionMentionEveryone))

	mainGetHandler := web.ControllerHandler(p.HandleRSS, "cp_rss")
	rssMux.Handle(pat.Get("/"), mainGetHandler)
	rssMux.Handle(pat.Get(""), mainGetHandler)

	addHandler := web.ControllerPostHandler(p.HandleNew, mainGetHandler, RSSFeedForm{})
	rssMux.Handle(pat.Post(""), addHandler)
	rssMux.Handle(pat.Post("/"), addHandler)
	rssMux.Handle(pat.Post("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
	rssMux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(p.HandleEdit), mainGetHandler, RSSFeedForm{}))
}

type RSSFeedForm struct {
	FeedURL         string
	DiscordChannel  int64 `valid:"channel,false"`
	MentionEveryone bool
	MentionRoles    []int64
	Enabled         bool
}

func (p *Plugin) HandleRSS(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	subs, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.GuildID.EQ(activeGuild.ID),
		qm.OrderBy("id DESC"),
	).AllG(ctx)
	if err != nil {
		return templateData, err
	}

	feedItems := make([]*FeedWithStatus, 0, len(subs))
	for _, sub := range subs {
		status, statusClass := feedStatus(sub)
		feedItems = append(feedItems, &FeedWithStatus{
			RSSFeedSubscription: sub,
			Status:              status,
			StatusClass:         statusClass,
		})
	}

	templateData["FeedItems"] = feedItems
	templateData["FreeLimit"] = GuildMaxRSSFeedsFree
	templateData["PremiumLimit"] = GuildMaxRSSFeedsPremium
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/rss"
	return templateData, nil
}

func (p *Plugin) HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	data := ctx.Value(common.ContextKeyParsedForm).(*RSSFeedForm)

	enabledLimit := MaxRSSFeedsForContext(ctx)
	enabledCount, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.GuildID.EQ(activeGuild.ID),
		models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
	).CountG(ctx)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed to check current enabled RSS feed count.")), err
	}
	if enabledCount >= int64(enabledLimit) {
		if premium.ContextPremium(ctx) {
			return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("You can only have up to %d enabled RSS feeds per server (premium limit). Disable or delete an existing feed to add a new one.", enabledLimit))), nil
		} else {
			return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("You can only have up to %d enabled RSS feeds per server (free limit). Upgrade to premium for more, or disable/delete an existing feed.", enabledLimit))), nil
		}
	}

	feedURL := strings.TrimSpace(data.FeedURL)
	fetchCtx, cancel := context.WithTimeout(ctx, feedFetchTimeout)
	defer cancel()

	res, err := fetchFeed(fetchCtx, feedURL, nil)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Couldn't read an RSS/Atom/JSON feed from that URL: %v", err))), nil
	}

	mentionRoles := data.MentionRoles
	if len(mentionRoles) == 0 {
		roles := r.Form["MentionRoles"]
		for _, roleStr := range roles {
			if roleID, err := strconv.ParseInt(roleStr, 10, 64); err == nil {
				mentionRoles = append(mentionRoles, roleID)
			}
		}
	}

	sub := &models.RSSFeedSubscription{
		GuildID:         activeGuild.ID,
		ChannelID:       data.DiscordChannel,
		FeedURL:         feedURL,
		MentionEveryone: data.MentionEveryone,
		MentionRoles:    mentionRoles,
		Enabled:         true,
	}
	if err := sub.InsertG(ctx, boil.Infer()); err != nil {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Failed to add RSS feed: %v", err))), err
	}

	// what's already in the feed isn't news, without this the first poll would post
	// everything published in the last day in one go
	if err := SeedSeenItems(sub.ID, res.Feed); err != nil {
		logger.WithError(err).WithField("feed_id", sub.ID).Warn("Failed seeding RSS deduplication set for new feed")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAddedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: data.FeedURL}))
	return templateData, nil
}

// FeedWithStatus is a subscription plus what the poller last saw when fetching it
type FeedWithStatus struct {
	*models.RSSFeedSubscription
	Status      string
	StatusClass string
}

func feedStatus(sub *models.RSSFeedSubscription) (status string, class string) {
	state, err := LoadFeedState(strings.TrimSpace(sub.FeedURL))
	if err != nil {
		return "Unknown", "text-muted"
	}

	lastError := common.CutStringShort(state.LastError, 200)

	switch {
	case !sub.Enabled && lastError != "":
		return "Disabled: " + lastError, "text-danger"
	case !sub.Enabled:
		return "Disabled", "text-muted"
	case state.CooldownUntil.After(time.Now()):
		return "Rate limited by the feed host, retrying " + common.HumanizeTime(common.DurationPrecisionMinutes, state.CooldownUntil), "text-warning"
	case !state.FirstFailure.IsZero():
		return fmt.Sprintf("Failing since %s: %s", common.HumanizeTime(common.DurationPrecisionMinutes, state.FirstFailure), lastError), "text-warning"
	case !state.LastSuccess.IsZero():
		return "Checked " + common.HumanizeTime(common.DurationPrecisionMinutes, state.LastSuccess), "text-success"
	}

	return "Waiting for the first check", "text-muted"
}

type ContextKey int

const (
	ContextKeySub ContextKey = iota
)

func BaseEditHandler(inner web.ControllerHandlerFunc) web.ControllerHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
		ctx := r.Context()
		activeGuild, templateData := web.GetBaseCPContextData(ctx)

		id, err := strconv.Atoi(pat.Param(r, "item"))
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Invalid feed ID")), err
		}

		sub, err := models.FindRSSFeedSubscriptionG(ctx, id)
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Failed retrieving that feed item")), err
		}

		if sub.GuildID != activeGuild.ID {
			return templateData.AddAlerts(web.ErrorAlert("This appears to belong somewhere else...")), nil
		}

		ctx = context.WithValue(ctx, ContextKeySub, sub)
		return inner(w, r.WithContext(ctx))
	}
}

func (p *Plugin) HandleEdit(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, templateData := web.GetBaseCPContextData(ctx)
	sub := ctx.Value(ContextKeySub).(*models.RSSFeedSubscription)
	data := ctx.Value(common.ContextKeyParsedForm).(*RSSFeedForm)

	if !sub.Enabled && data.Enabled {
		enabledLimit := MaxRSSFeedsForContext(ctx)
		enabledCount, err := models.RSSFeedSubscriptions(
			models.RSSFeedSubscriptionWhere.GuildID.EQ(sub.GuildID),
			models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
		).CountG(ctx)
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Failed to check current enabled RSS feed count.")), err
		}
		if enabledCount >= int64(enabledLimit) {
			if premium.ContextPremium(ctx) {
				return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("You can only have up to %d enabled RSS feeds per server (premium limit). Disable or delete an existing feed to enable this one.", enabledLimit))), nil
			} else {
				return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("You can only have up to %d enabled RSS feeds per server (free limit). Upgrade to premium for more, or disable/delete an existing feed.", enabledLimit))), nil
			}
		}
	}

	reEnabled := !sub.Enabled && data.Enabled

	sub.ChannelID = data.DiscordChannel
	sub.MentionEveryone = data.MentionEveryone
	sub.MentionRoles = data.MentionRoles
	sub.Enabled = data.Enabled

	_, err := sub.UpdateG(ctx, boil.Whitelist("channel_id", "enabled", "mention_everyone", "mention_roles"))
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed to update RSS feed.")), err
	}

	if reEnabled {
		// give a feed that was turned back on a clean slate to fail from
		saveFeedState(strings.TrimSpace(sub.FeedURL), nil, "first_failure", "last_error", "cooldown_until")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.FeedURL}))
	return templateData, nil
}

func (p *Plugin) HandleRemove(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, templateData := web.GetBaseCPContextData(ctx)

	sub := ctx.Value(ContextKeySub).(*models.RSSFeedSubscription)
	_, err := sub.DeleteG(ctx)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed to remove RSS feed.")), err
	}

	key := seenSetKey(sub.ID)
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", key))
	if err != nil {
		logger.WithError(err).WithField("key", key).Warn("Failed to delete RSS deduplication key after feed deletion")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyRemovedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.FeedURL}))
	return templateData, nil
}

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "RSS feeds"
	templateData["SettingsPath"] = "/rss"

	numFeeds, err := models.RSSFeedSubscriptions(models.RSSFeedSubscriptionWhere.GuildID.EQ(ag.ID), models.RSSFeedSubscriptionWhere.Enabled.EQ(true)).CountG(r.Context())
	if err != nil {
		return templateData, err
	}

	if numFeeds > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<p>Active RSS feeds: <code>%d</code></p>`
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, numFeeds))

	return templateData, nil
}
