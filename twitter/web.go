package twitter

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/twitter/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

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
}

func (p *Plugin) InitWeb() {

	web.LoadHTMLTemplate("../../twitter/assets/twitter.html", "templates/plugins/twitter.html")
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "Twitter Feeds",
		URL:  "twitter",
		Icon: "fab fa-twitter",
	})

	mux := goji.SubMux()
	web.CPMux.Handle(pat.New("/twitter/*"), mux)
	web.CPMux.Handle(pat.New("/twitter"), mux)

	// Alll handlers here require guild channels present
	mux.Use(web.RequireGuildChannelsMiddleware)

	mainGetHandler := web.ControllerHandler(p.HandleTwitter, "cp_twitter")

	mux.Handle(pat.Get("/"), mainGetHandler)
	mux.Handle(pat.Get(""), mainGetHandler)

	addHandler := web.ControllerPostHandler(p.HandleNew, mainGetHandler, Form{}, "Added a new twitter feed")

	mux.Handle(pat.Post(""), addHandler)
	mux.Handle(pat.Post("/"), addHandler)
	mux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(p.HandleEdit), mainGetHandler, EditForm{}, "Updated a twitter feed"))
	mux.Handle(pat.Post("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil, "Removed a twitter feed"))
	mux.Handle(pat.Get("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil, "Removed a twitter feed"))
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

	if !premium.ContextPremium(ctx) {
		return templateData.AddAlerts(web.ErrorAlert("Twitter feeds are premium only")), nil
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
	sub.Enabled = true
	sub.IncludeRT = data.IncludeRetweets
	sub.IncludeReplies = data.IncludeReplies

	_, err = sub.UpdateG(ctx, boil.Whitelist("channel_id", "enabled", "include_replies", "include_rt"))
	return
}

func (p *Plugin) HandleRemove(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	_, templateData = web.GetBaseCPContextData(ctx)

	sub := ctx.Value(ContextKeySub).(*models.TwitterFeed)
	_, err = sub.DeleteG(ctx)
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
