package youtube

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/youtube/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"google.golang.org/api/youtube/v3"
)

type CustomYoutubeAnnouncement struct {
	GuildID      int64                             `json:"guild_id"`
	Subscription models.YoutubeChannelSubscription `json:"subscription"`
	Video        youtube.Video                     `json:"video"`
}

func (p *Plugin) BotInit() {
	pubsub.AddHandler("custom_youtube_announcement", func(evt *pubsub.Event) {
		if evt.Data == nil {
			return
		}
		data := evt.Data.(*CustomYoutubeAnnouncement)
		gs := bot.State.GetGuild(data.GuildID)
		if gs == nil {
			return
		}
		p.handleCustomAnnouncement(data)
	}, CustomYoutubeAnnouncement{})
}

func (p *Plugin) Status() (string, string) {
	var unique int
	common.RedisPool.Do(radix.Cmd(&unique, "ZCARD", RedisKeyWebSubChannels))

	total, _ := models.YoutubeChannelSubscriptions().CountG(context.Background())

	return "Unique/Total", fmt.Sprintf("%d/%d", unique, total)
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	toDisable, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(guildID)),
		models.YoutubeChannelSubscriptionWhere.Enabled.EQ(true),

		qm.Offset(GuildMaxEnabledFeeds),
		qm.OrderBy("id DESC"),
	).AllG(context.Background())

	for _, f := range toDisable {
		f.Enabled = false
		f.UpdateG(context.Background(), boil.Infer())
	}

	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed disabling excess feeds")
		return err
	}

	logger.WithField("guild", guildID).Infof("disabled %d excess feeds", len(toDisable))
	return nil
}

func (p *Plugin) handleCustomAnnouncement(notif *CustomYoutubeAnnouncement) {
	sub := notif.Subscription
	video := notif.Video

	logger.WithField("guild_id", sub.GuildID).WithField("channel_id", sub.ChannelID).WithField("video_id", video.Id).Info("handling custom youtube announcement")
	guildID, err := strconv.ParseInt(sub.GuildID, 10, 64)
	if err != nil {
		logger.WithError(err).WithField("guild_id", sub.GuildID).Error("failed parsing guild_id")
		return
	}

	channelID, err := strconv.ParseInt(sub.ChannelID, 10, 64)
	if err != nil {
		logger.WithError(err).WithField("channel_id", sub.ChannelID).Error("failed parsing channel_id")
		return
	}

	guildState := bot.State.GetGuild(guildID)
	if guildState == nil {
		logger.Errorf("guild_id %d not found in state for youtube feed", guildID)
		p.DisableGuildFeeds(guildID)
		return
	}

	channelState := guildState.GetChannel(channelID)
	if channelState == nil {
		logger.Errorf("channel_id %d for guild_id %d not found in state for youtube feed", channelID, guildID)
		p.DisableChannelFeeds(channelID)
		return
	}

	announcement, err := models.FindYoutubeAnnouncementG(context.Background(), guildID)
	hasCustomAnnouncement := true
	if err != nil {
		hasCustomAnnouncement = false
		if err == sql.ErrNoRows {
			logger.WithError(err).Debugf("Custom announcement doesn't exist for guild_id %d", guildID)
		} else {
			logger.WithError(err).Errorf("Failed fetching custom announcement for guild_id %d", guildID)
		}
	}

	if !hasCustomAnnouncement {
		return
	}

	ctx := templates.NewContext(guildState, channelState, nil)
	videoDurationString := strings.ToLower(strings.TrimPrefix(video.ContentDetails.Duration, "PT"))
	videoDuration, err := common.ParseDuration(videoDurationString)
	if err != nil {
		videoDuration = time.Duration(0)
	}

	videoUrl := "https://www.youtube.com/watch?v=" + video.Id
	ctx.Data["URL"] = videoUrl
	ctx.Data["ChannelName"] = sub.YoutubeChannelName
	ctx.Data["YoutubeChannelName"] = sub.YoutubeChannelName
	ctx.Data["YoutubeChannelID"] = sub.YoutubeChannelID
	ctx.Data["ChannelID"] = sub.ChannelID
	//should be true for upcoming too as upcoming is also technically a livestream
	ctx.Data["IsLiveStream"] = (video.Snippet.LiveBroadcastContent == "live" || video.Snippet.LiveBroadcastContent == "upcoming")
	ctx.Data["IsUpcoming"] = video.Snippet.LiveBroadcastContent == "upcoming"
	ctx.Data["VideoID"] = video.Id
	ctx.Data["VideoTitle"] = video.Snippet.Title
	ctx.Data["VideoThumbnail"] = fmt.Sprintf("https://img.youtube.com/vi/%s/maxresdefault.jpg", video.Id)
	ctx.Data["VideoDescription"] = video.Snippet.Description
	ctx.Data["VideoDurationSeconds"] = int(math.Round(videoDuration.Seconds()))
	//full video object in case people want to do more advanced stuff
	ctx.Data["Video"] = video
	publishAnnouncement := ctx.CurrentFrame.PublishResponse
	content, err := ctx.Execute(announcement.Message)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Errorf("custom announcement parsing failed")
		return
	}

	go analytics.RecordActiveUnit(guildID, p, "posted_youtube_message")
	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "youtube"}).Inc()
	if content == "" {
		return
	}

	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone}
	mqueue.QueueMessage(&mqueue.QueuedElement{
		GuildID:             guildID,
		ChannelID:           channelID,
		Source:              "youtube",
		SourceItemID:        "",
		MessageStr:          content,
		Priority:            2,
		PublishAnnouncement: publishAnnouncement,
		AllowedMentions: discordgo.AllowedMentions{
			Parse: parseMentions,
		},
	})
}
