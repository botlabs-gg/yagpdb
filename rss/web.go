package rss

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/rss/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/mmcdole/gofeed"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/rss.html
var PageHTML string

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
	rssMux.Handle(pat.Get("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
	rssMux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(p.HandleEdit), mainGetHandler, RSSFeedForm{}))
}

type RSSFeedForm struct {
	FeedURL         string
	DiscordChannel  int64 `valid:"channel,false"`
	FeedName        string
	MentionEveryone bool
	MentionRoles    []int64
	Enabled         bool
}

func (p *Plugin) HandleRSS(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	subs, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.GuildID.EQ(activeGuild.ID),
		models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
		qm.OrderBy("id DESC"),
	).AllG(ctx)
	if err != nil {
		return templateData, err
	}

	templateData["FeedItems"] = subs // FIX: use FeedItems to match template
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/rss"
	return templateData, nil
}

func (p *Plugin) HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	data := ctx.Value(common.ContextKeyParsedForm).(*RSSFeedForm)

	if !premium.ContextPremium(ctx) {
		return templateData.AddAlerts(web.ErrorAlert("RSS feeds are a premium-only feature.")), nil
	}

	count, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.GuildID.EQ(activeGuild.ID),
		models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
	).CountG(ctx)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed to check current RSS feed count.")), err
	}
	if count >= 5 {
		return templateData.AddAlerts(web.ErrorAlert("You can only have up to 5 RSS feeds per server.")), nil
	}

	parser := gofeed.NewParser()
	_, err = parser.ParseURL(data.FeedURL)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("The provided URL does not contain a valid RSS/Atom/JSON feed.")), nil
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
		FeedURL:         data.FeedURL,
		MentionEveryone: data.MentionEveryone,
		MentionRoles:    mentionRoles,
		Enabled:         true,
	}
	if err := sub.InsertG(ctx, boil.Infer()); err != nil {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Failed to add RSS feed: %v", err))), err
	}
	return templateData, nil
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

	sub.ChannelID = data.DiscordChannel
	sub.MentionEveryone = data.MentionEveryone
	sub.MentionRoles = data.MentionRoles
	sub.Enabled = data.Enabled

	_, err := sub.UpdateG(ctx, boil.Infer())
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed to update RSS feed.")), err
	}
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
	return templateData, nil
}
