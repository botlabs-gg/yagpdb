package twitch

import (
	"context"
	"fmt"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/twitch/models"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type CustomTwitchAnnouncement struct {
	GuildID      int64                            `json:"guild_id"`
	Subscription models.TwitchChannelSubscription `json:"subscription"`
	Stream       helix.Stream                     `json:"stream"`
	IsLive       bool                             `json:"is_live"`
	VODUrl       string                           `json:"vod_url"`
}

func (p *Plugin) BotInit() {
	pubsub.AddHandler("custom_twitch_announcement", func(evt *pubsub.Event) {
		if evt.Data == nil {
			return
		}
		data := evt.Data.(*CustomTwitchAnnouncement)
		p.handleCustomAnnouncement(data)
	}, CustomTwitchAnnouncement{})
}

func (p *Plugin) Status() (string, string) {
	total, _ := models.TwitchChannelSubscriptions().CountG(context.Background())
	return "Total Subs", fmt.Sprintf("%d", total)
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	ctx := context.Background()

	// 1. Disable Custom Announcements (Premium Only)
	announcement, err := models.FindTwitchAnnouncementG(ctx, guildID)
	if err == nil {
		announcement.Enabled = false
		_, err = announcement.UpdateG(ctx, boil.Whitelist("enabled"))
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("failed disabling custom twitch announcement")
		}
	}

	// 2. Enforce Feed Limits (Free Limit)
	// Fetch all enabled subscriptions ordered by ID DESC
	subs, err := models.TwitchChannelSubscriptions(
		models.TwitchChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(guildID)),
		models.TwitchChannelSubscriptionWhere.Enabled.EQ(true),
		qm.OrderBy("id DESC"),
	).AllG(ctx)

	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed retrieving twitch subscriptions for limit enforcement")
		return err
	}

	if len(subs) > GuildMaxEnabledFeeds {
		// Disable excess feeds
		excessSubs := subs[GuildMaxEnabledFeeds:]
		for _, sub := range excessSubs {
			sub.Enabled = false
			_, err := sub.UpdateG(ctx, boil.Whitelist("enabled"))
			if err != nil {
				logger.WithError(err).WithField("guild", guildID).WithField("sub_id", sub.ID).Error("failed disabling excess twitch subscription")
			}
		}
	}

	return nil
}

func (p *Plugin) handleCustomAnnouncement(notif *CustomTwitchAnnouncement) {
	sub := notif.Subscription
	stream := notif.Stream
	guildID := notif.GuildID
	isLive := notif.IsLive
	vodUrl := notif.VODUrl

	logger.WithField("guild_id", guildID).WithField("channel_id", sub.ChannelID).WithField("stream_id", stream.ID).WithField("is_live", isLive).Info("handling custom twitch announcement")

	guildState := bot.State.GetGuild(guildID)
	if guildState == nil {
		return
	}

	channelID, _ := strconv.ParseInt(sub.ChannelID, 10, 64)
	channelState := guildState.GetChannel(channelID)
	if channelState == nil {
		return
	}

	announcement, err := models.FindTwitchAnnouncementG(context.Background(), guildID)
	if err != nil {
		return
	}

	if !announcement.Enabled {
		return
	}

	ctx := templates.NewContext(guildState, channelState, nil)

	ctx.Data["URL"] = "https://twitch.tv/" + stream.UserLogin
	ctx.Data["User"] = stream.UserName
	ctx.Data["Title"] = stream.Title
	ctx.Data["Game"] = stream.GameName
	ctx.Data["Stream"] = stream
	ctx.Data["IsLive"] = isLive
	ctx.Data["VODUrl"] = vodUrl
	if isLive {
		ctx.Data["Status"] = "live"
	} else {
		ctx.Data["Status"] = "offline"
	}

	content, err := ctx.Execute(announcement.Message)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Errorf("custom announcement parsing failed")
		return
	}

	if content == "" {
		return
	}

	go analytics.RecordActiveUnit(guildID, p, "posted_twitch_message")
	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "twitch"}).Inc()

	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone}
	mqueue.QueueMessage(&mqueue.QueuedElement{
		GuildID:             guildID,
		ChannelID:           channelID,
		Source:              "twitch",
		SourceItemID:        stream.ID,
		MessageStr:          content,
		Priority:            2,
		PublishAnnouncement: false,
		AllowedMentions: discordgo.AllowedMentions{
			Parse: parseMentions,
		},
	})
}
