package youtube

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/botlabs-gg/yagpdb/v2/youtube/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/youtube.html
var PageHTML string

var (
	panelLogKeyAddedFeed    = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "youtube_added_feed", FormatString: "Added youtube feed from %s"})
	panelLogKeyAnnouncement = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "youtube_announcement", FormatString: "Updated YouTube Announcement"})
	panelLogKeyRemovedFeed  = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "youtube_removed_feed", FormatString: "Removed youtube feed from %s"})
	panelLogKeyUpdatedFeed  = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "youtube_updated_feed", FormatString: "Updated youtube feed from %s"})
)

type YoutubeFeedForm struct {
	YoutubeUrl        string
	DiscordChannel    int64 `valid:"channel,false"`
	ID                uint
	MentionEveryone   bool
	MentionRoles      []int64
	PublishShorts     bool
	PublishLivestream bool
	Enabled           bool
	CustomMessage     string
}

type YoutubeAnnouncementForm struct {
	Message string `json:"message" valid:"template,5000"`
	Enabled bool
}

var (
	ytVideoIDRegex    = regexp.MustCompile(`\A[\w\-]+\z`)
	ytPlaylistIDRegex = regexp.MustCompile(`\A(?:PL|OLAK|RDCLAK)[-_0-9A-Za-z]+\z`)
	ytChannelIDRegex  = regexp.MustCompile(`\AUC[\w\-]{21}[AQgw]\z`)
	ytHandleRegex     = regexp.MustCompile(`\A@[\w\-.]{3,30}\z`)
)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("youtube/assets/youtube.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "Youtube",
		URL:  "youtube",
		Icon: "fab fa-youtube",
	})

	ytMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/youtube/*"), ytMux)
	web.CPMux.Handle(pat.New("/youtube"), ytMux)

	// All handlers here require guild channels present
	ytMux.Use(web.RequireBotMemberMW)
	ytMux.Use(web.RequirePermMW(discordgo.PermissionMentionEveryone))

	mainGetHandler := web.ControllerHandler(p.HandleYoutube, "cp_youtube")

	ytMux.Handle(pat.Get("/"), mainGetHandler)
	ytMux.Handle(pat.Get(""), mainGetHandler)

	addHandler := web.ControllerPostHandler(p.HandleNew, mainGetHandler, YoutubeFeedForm{})

	ytMux.Handle(pat.Post(""), addHandler)
	ytMux.Handle(pat.Post("/"), addHandler)
	ytMux.Handle(pat.Post("/announcement"), web.ControllerPostHandler(p.HandleYoutubeAnnouncement, mainGetHandler, YoutubeAnnouncementForm{}))
	ytMux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(p.HandleEdit), mainGetHandler, YoutubeFeedForm{}))
	ytMux.Handle(pat.Post("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))
	ytMux.Handle(pat.Get("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(p.HandleRemove), mainGetHandler, nil))

	// The handler from pubsubhub
	web.RootMux.Handle(pat.New("/yt_new_upload/"+confWebsubVerifytoken.GetString()), http.HandlerFunc(p.HandleFeedUpdate))
}

func (p *Plugin) HandleYoutube(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	subs, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)),
		models.YoutubeChannelSubscriptionWhere.Enabled.EQ(true),
		qm.OrderBy("id DESC"),
	).AllG(ctx)
	if err != nil && err != sql.ErrNoRows {
		return templateData, err
	}

	announcement, err := models.FindYoutubeAnnouncementG(ctx, activeGuild.ID)
	if err != nil {
		announcement = &models.YoutubeAnnouncement{
			GuildID: activeGuild.ID,
			Message: "{{.YoutubeChannelName}} published a new video! {{.URL}}",
			Enabled: false,
		}
	}

	templateData["Announcement"] = announcement
	templateData["Subs"] = subs
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/youtube"
	return templateData, nil
}

func (p *Plugin) HandleYoutubeAnnouncement(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	form := ctx.Value(common.ContextKeyParsedForm).(*YoutubeAnnouncementForm)

	announcement := &models.YoutubeAnnouncement{
		GuildID: activeGuild.ID,
		Message: form.Message,
		Enabled: form.Enabled,
	}
	err = announcement.UpsertG(ctx, true, []string{"guild_id"},
		boil.Whitelist("message", "enabled"), /* updateColumns */
		boil.Whitelist("guild_id", "message", "enabled") /* insertColumns */)
	if err != nil {
		return templateData, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAnnouncement, &cplogs.Param{}))
	return templateData, nil
}

func (p *Plugin) HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	totalFeeds, _ := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)),
	).CountG(ctx)

	if int(totalFeeds) >= GuildMaxFeeds {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max allowed %d youtube feeds allowed", GuildMaxFeeds))), nil
	}

	totalEnabled, _ := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)),
		models.YoutubeChannelSubscriptionWhere.Enabled.EQ(true),
	).CountG(ctx)

	if int(totalEnabled) >= MaxFeedsEnabledForContext(ctx) {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max Enabled %d youtube feeds allowed (%d for premium servers)", GuildMaxEnabledFeeds, GuildMaxEnabledFeedsPremium))), nil
	}

	data := ctx.Value(common.ContextKeyParsedForm).(*YoutubeFeedForm)
	channelUrl := data.YoutubeUrl
	parsedUrl, err := url.Parse(channelUrl)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Invalid link <b>%s<b>, make sure it is a valid youtube url", channelUrl))), err
	}

	logger.Debugf("Got Request for new youtube URL add: %s Guild: %d ", channelUrl, activeGuild.ID)

	id, err := p.parseYtUrl(parsedUrl)
	if err != nil {
		logger.WithError(err).Errorf("error occured parsing channel from url %q", channelUrl)
		return templateData.AddAlerts(web.ErrorAlert(err)), err
	}

	list := p.YTService.Channels.List(listParts).MaxResults(1)
	cResp, err := id.getChannelList(p, list)
	if cResp != nil && len(cResp.Items) < 1 {
		err = ErrNoChannel
	}
	if err != nil {
		logger.WithError(err).Errorf("error occurred fetching channel for url %s", channelUrl)
		return templateData.AddAlerts(web.ErrorAlert("No channel found for that link")), err
	}

	ytChannel := cResp.Items[0]
	sub, err := p.AddFeed(activeGuild.ID, data.DiscordChannel, ytChannel, data.MentionEveryone, data.PublishLivestream, data.PublishShorts, data.MentionRoles)

	if err != nil {
		if err == ErrNoChannel {
			return templateData.AddAlerts(web.ErrorAlert("No channel by that id/username found")), errors.New("channel not found")
		}
		return templateData, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAddedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.YoutubeChannelName}))
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

		// Get the actual watch item from the config
		sub, err := models.FindYoutubeChannelSubscriptionG(ctx, id)
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

func (p *Plugin) HandleEdit(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	updatedSub := ctx.Value(ContextKeySub).(*models.YoutubeChannelSubscription)
	form := ctx.Value(common.ContextKeyParsedForm).(*YoutubeFeedForm)

	updatedSub.MentionEveryone = form.MentionEveryone
	updatedSub.MentionRoles = form.MentionRoles
	updatedSub.PublishLivestream = form.PublishLivestream
	updatedSub.PublishShorts = form.PublishShorts
	updatedSub.ChannelID = discordgo.StrID(form.DiscordChannel)
	updatedSub.Enabled = form.Enabled

	numEnabled, _ := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)),
		models.YoutubeChannelSubscriptionWhere.Enabled.EQ(true),
	).CountG(ctx)
	if int(numEnabled) >= MaxFeedsEnabledForContext(ctx) {
		curSub, err := models.FindYoutubeChannelSubscriptionG(ctx, updatedSub.ID)
		if err != nil {
			logger.WithError(err).Errorf("Failed getting feed %d", updatedSub.ID)
		}
		if !curSub.Enabled && updatedSub.Enabled {
			return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max %d enabled youtube feeds allowed (%d for premium servers)", GuildMaxEnabledFeeds, GuildMaxEnabledFeedsPremium))), nil
		}
	}

	_, err = updatedSub.UpdateG(ctx, boil.Infer())
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: updatedSub.YoutubeChannelName}))
	}
	return templateData, err
}

func (p *Plugin) HandleRemove(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	_, templateData = web.GetBaseCPContextData(ctx)

	sub := ctx.Value(ContextKeySub).(*models.YoutubeChannelSubscription)
	_, err = sub.DeleteG(ctx)
	if err != nil {
		return
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyRemovedFeed, &cplogs.Param{Type: cplogs.ParamTypeString, Value: sub.YoutubeChannelName}))

	p.MaybeRemoveChannelWatch(sub.YoutubeChannelID)
	return
}

func (p *Plugin) HandleFeedUpdate(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ctx := r.Context()
	switch query.Get("hub.mode") {
	case "subscribe":
		if query.Get("hub.verify_token") != confWebsubVerifytoken.GetString() {
			return // We don't want no intruders here
		}

		web.CtxLogger(ctx).Info("Responding to challenge: ", query.Get("hub.challenge"))
		p.ValidateSubscription(w, r, query)
		return
	case "unsubscribe":
		if query.Get("hub.verify_token") != confWebsubVerifytoken.GetString() {
			return // We don't want no intruders here
		}

		w.Write([]byte(query.Get("hub.challenge")))

		topicURI, err := url.ParseRequestURI(query.Get("hub.topic"))
		if err != nil {
			web.CtxLogger(ctx).WithError(err).Error("Failed parsing websub topic URI")
			return
		}

		common.RedisPool.Do(radix.Cmd(nil, "ZREM", RedisKeyWebSubChannels, topicURI.Query().Get("channel_id")))
		return
	}

	// Handle new/updated video
	defer r.Body.Close()
	bodyReader := io.LimitReader(r.Body, 0xffff1)

	result, err := io.ReadAll(bodyReader)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("Failed reading body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var parsed XMLFeed

	err = xml.Unmarshal(result, &parsed)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("Failed parsing feed body: ", string(result))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = p.CheckVideo(parsed)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("Failed parsing checking new youtube video")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (p *Plugin) ValidateSubscription(w http.ResponseWriter, r *http.Request, query url.Values) {
	w.Write([]byte(query.Get("hub.challenge")))

	lease := query.Get("hub.lease_seconds")
	if lease != "" {
		parsed, err := strconv.ParseInt(lease, 10, 64)
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).Error("Failed parsing websub lease time")
			return
		}

		expires := time.Now().Add(time.Second * time.Duration(parsed-10)).Unix()

		topicURI, err := url.ParseRequestURI(query.Get("hub.topic"))
		if err != nil {
			web.CtxLogger(r.Context()).WithError(err).Error("Failed parsing websub topic URI")
			return
		}

		common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", RedisKeyWebSubChannels, expires, topicURI.Query().Get("channel_id")))
	}
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Youtube feeds"
	templateData["SettingsPath"] = "/youtube"

	numFeeds, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(activeGuild.ID)),
		models.YoutubeChannelSubscriptionWhere.Enabled.EQ(true),
	).CountG(r.Context())
	if numFeeds > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<p>Active Youtube feeds: <code>%d</code></p>`
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, numFeeds))

	return templateData, err
}
