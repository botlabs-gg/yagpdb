package twitter

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/go-twitter/twitter"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/twitter/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/twitter.html
var PageHTML string

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

type Form struct {
	TwitterUser    string `valid:",1,256"`
	DiscordChannel int64  `valid:"channel,false"`
	ID             int64
}

type EditForm struct {
	DiscordChannel  int64 `valid:"channel,false"`
	IncludeReplies  bool
	IncludeRetweets bool
	Enabled         bool
}

var (
	panelLogKeyAddedFeed   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitter_added_feed", FormatString: "Added twitter feed from %s"})
	panelLogKeyRemovedFeed = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitter_removed_feed", FormatString: "Removed twitter feed from %s"})
	panelLogKeyUpdatedFeed = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "twitter_updated_feed", FormatString: "Updated twitter feed from %s"})
)

func (p *Plugin) InitWeb() {

	web.AddHTMLTemplate("twitter/assets/twitter.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "Twitter Feeds",
		URL:  "twitter",
		Icon: "fab fa-twitter",
	})

	mux := goji.SubMux()
	mux.Use(web.RequireBotMemberMW)
	mux.Use(web.RequirePermMW(discordgo.PermissionManageWebhooks))
	web.CPMux.Handle(pat.New("/twitter/*"), mux)
	web.CPMux.Handle(pat.New("/twitter"), mux)

	mainGetHandler := web.ControllerHandler(p.HandleTwitter, "cp_twitter")

	mux.Handle(pat.Get("/"), mainGetHandler)
	mux.Handle(pat.Get(""), mainGetHandler)

	addHandler := web.ControllerPostHandler(p.HandleNew, mainGetHandler, Form{})

	mux.Handle(pat.Post(""), addHandler)
	mux.Handle(pat.Post("/"), addHandler)
	mux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(p.HandleEdit), mainGetHandler, EditForm{}))
	mux.Handle(pat.Post("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
	mux.Handle(pat.Get("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
}

func (p *Plugin) HandleTwitter(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	ag, templateData := web.GetBaseCPContextData(ctx)

	result, err := models.TwitterFeeds(models.TwitterFeedWhere.GuildID.EQ(ag.ID), qm.OrderBy("id asc")).AllG(ctx)
	if err != nil {
		return templateData, err
	}

	templateData["FeedItems"] = result

	return templateData, nil
}

func (p *Plugin) HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	if premium.ContextPremiumTier(ctx) != premium.PremiumTierPremium {
		return templateData.AddAlerts(web.ErrorAlert("Twitter feeds are paid premium only")), nil
	}

	// limit it to max 25 feeds
	currentCount, err := models.TwitterFeeds(models.TwitterFeedWhere.GuildID.EQ(activeGuild.ID)).CountG(ctx)
	if err != nil {
		return templateData, err
	}

	if currentCount >= 25 {
		return templateData.AddAlerts(web.ErrorAlert("Max 25 feeds per server")), nil
	}

	globalCount, err := models.TwitterFeeds(models.TwitterFeedWhere.GuildID.EQ(activeGuild.ID)).CountG(ctx)
	if err != nil {
		return templateData, err
	}

	if globalCount >= 4000 {
		return templateData.AddAlerts(web.ErrorAlert("Bot hit max feeds, contact bot owner")), nil
	}

	form := ctx.Value(common.ContextKeyParsedForm).(*Form)

	// search up the ID
	users, _, err := p.twitterAPI.Users.Lookup(&twitter.UserLookupParams{
		ScreenName: []string{form.TwitterUser},
	})
	if err != nil {
		if cast, ok := err.(twitter.APIError); ok {
			if cast.Errors[0].Code == 17 {
				return templateData.AddAlerts(web.ErrorAlert("User not found")), nil
			}
		}
		return templateData, err
	}

	if len(users) < 1 {
		return templateData.AddAlerts(web.ErrorAlert("User not found")), nil
	}

	user := users[0]

	m := &models.TwitterFeed{
		GuildID:         activeGuild.ID,
		TwitterUsername: user.ScreenName,
		TwitterUserID:   user.ID,
		ChannelID:       form.DiscordChannel,
		Enabled:         true,
	}

	err = m.InsertG(ctx, boil.Infer())
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAddedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: user.ScreenName}))
	}
	return templateData, err
}

type ContextKey int

const (
	ContextKeySub ContextKey = iota
)

func BaseEditHandler(inner web.ControllerHandlerFunc) web.ControllerHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
		ctx := r.Context()
		activeGuild, templateData := web.GetBaseCPContextData(ctx)

		id, _ := strconv.Atoi(pat.Param(r, "item"))

		// Get tha actual watch item from the config
		feedItem, err := models.FindTwitterFeedG(ctx, int64(id))
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Failed retrieving that feed item")), err
		}

		if feedItem.GuildID != activeGuild.ID {
			return templateData.AddAlerts(web.ErrorAlert("This appears to belong somewhere else...")), nil
		}

		ctx = context.WithValue(ctx, ContextKeySub, feedItem)

		return inner(w, r.WithContext(ctx))
	}
}

func (p *Plugin) HandleEdit(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	_, templateData = web.GetBaseCPContextData(ctx)

	if !premium.ContextPremium(ctx) {
		return templateData.AddAlerts(web.ErrorAlert("Twitter feeds are premium only")), nil
	}

	sub := ctx.Value(ContextKeySub).(*models.TwitterFeed)
	data := ctx.Value(common.ContextKeyParsedForm).(*EditForm)

	sub.ChannelID = data.DiscordChannel
	sub.Enabled = data.Enabled
	sub.IncludeRT = data.IncludeRetweets
	sub.IncludeReplies = data.IncludeReplies

	_, err = sub.UpdateG(ctx, boil.Whitelist("channel_id", "enabled", "include_replies", "include_rt"))
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.TwitterUsername}))
	}
	return
}

func (p *Plugin) HandleRemove(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	_, templateData = web.GetBaseCPContextData(ctx)

	sub := ctx.Value(ContextKeySub).(*models.TwitterFeed)
	_, err = sub.DeleteG(ctx)
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyRemovedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.TwitterUsername}))
	}
	return templateData, err
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Twitter feeds"
	templateData["SettingsPath"] = "/twitter"

	numFeeds, err := models.TwitterFeeds(models.TwitterFeedWhere.GuildID.EQ(ag.ID)).CountG(r.Context())
	if err != nil {
		return templateData, err
	}

	if numFeeds > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<p>Active Twitter feeds: <code>%d</code></p>`
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, numFeeds))

	return templateData, nil
}
