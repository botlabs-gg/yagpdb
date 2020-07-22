package youtube

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

const (
	MaxChannelsPerPoll  = 30
	PollInterval        = time.Second * 10
	WebSubCheckInterval = time.Second * 10
	// PollInterval = time.Second * 5 // <- used for debug purposes
)

func (p *Plugin) StartFeed() {
	p.Stop = make(chan *sync.WaitGroup)
	p.runWebsubChecker()
}

func (p *Plugin) StopFeed(wg *sync.WaitGroup) {

	if p.Stop != nil {
		p.Stop <- wg
	} else {
		wg.Done()
	}
}

func (p *Plugin) SetupClient() error {
	httpClient, err := google.DefaultClient(context.Background(), youtube.YoutubeScope)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	yt, err := youtube.New(httpClient)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	p.YTService = yt

	return nil
}

// keeps the subscriptions up to date by updating the ones soon to be expiring
func (p *Plugin) runWebsubChecker() {
	p.syncWebSubs()

	websubTicker := time.NewTicker(WebSubCheckInterval)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-websubTicker.C:
			p.checkExpiringWebsubs()
		}
	}
}

func (p *Plugin) checkExpiringWebsubs() {
	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 10)
	if err != nil {
		logger.WithError(err).Error("Failed locking channels lock")
		return
	}
	defer common.UnlockRedisKey(RedisChannelsLockKey)

	maxScore := time.Now().Unix()

	var expiring []string
	err = common.RedisPool.Do(radix.FlatCmd(&expiring, "ZRANGEBYSCORE", RedisKeyWebSubChannels, "-inf", maxScore))
	if err != nil {
		logger.WithError(err).Error("Failed checking websubs")
		return
	}

	for _, v := range expiring {
		err := p.WebSubSubscribe(v)
		if err != nil {
			logger.WithError(err).WithField("yt_channel", v).Error("Failed subscribing to channel")
		}
		time.Sleep(time.Second)
	}
}

func (p *Plugin) syncWebSubs() {
	var activeChannels []string
	err := common.SQLX.Select(&activeChannels, "SELECT DISTINCT(youtube_channel_id) FROM youtube_channel_subscriptions;")
	if err != nil {
		logger.WithError(err).Error("Failed syncing websubs, failed retrieving subbed channels")
		return
	}

	common.RedisPool.Do(radix.WithConn(RedisKeyWebSubChannels, func(client radix.Conn) error {

		locked := false

		for _, channel := range activeChannels {
			if !locked {
				err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5000)
				if err != nil {
					logger.WithError(err).Error("Failed locking channels lock")
					return err
				}
				locked = true
			}

			mn := radix.MaybeNil{}
			client.Do(radix.Cmd(&mn, "ZSCORE", RedisKeyWebSubChannels, channel))
			if mn.Nil {
				// Not added
				err := p.WebSubSubscribe(channel)
				if err != nil {
					logger.WithError(err).WithField("yt_channel", channel).Error("Failed subscribing to channel")
				}

				common.UnlockRedisKey(RedisChannelsLockKey)
				locked = false

				time.Sleep(time.Second)
			}
		}

		if locked {
			common.UnlockRedisKey(RedisChannelsLockKey)
		}

		return nil
	}))
}

func (p *Plugin) removeAllSubsForChannel(channel string) {
	err := common.GORM.Where("youtube_channel_id = ?", channel).Delete(ChannelSubscription{}).Error
	if err != nil {
		logger.WithError(err).WithField("yt_channel", channel).Error("failed removing channel")
	}
	go p.MaybeRemoveChannelWatch(channel)
}

func (p *Plugin) sendNewVidMessage(guild, discordChannel string, channelTitle string, videoID string, mentionEveryone bool) {
	content := fmt.Sprintf("**%s** uploaded a new youtube video!\n%s", channelTitle, "https://www.youtube.com/watch?v="+videoID)
	if mentionEveryone {
		content += " @everyone"
	}

	parsedChannel, _ := strconv.ParseInt(discordChannel, 10, 64)
	parsedGuild, _ := strconv.ParseInt(guild, 10, 64)

	parseMentions := []discordgo.AllowedMentionType{}
	if mentionEveryone {
		parseMentions = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeEveryone}
	}

	go analytics.RecordActiveUnit(parsedGuild, p, "posted_youtube_message")
	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "youtube"}).Inc()

	mqueue.QueueMessage(&mqueue.QueuedElement{
		Guild:      parsedGuild,
		Channel:    parsedChannel,
		Source:     "youtube",
		SourceID:   "",
		MessageStr: content,
		Priority:   2,
		AllowedMentions: discordgo.AllowedMentions{
			Parse: parseMentions,
		},
	})
}

var (
	ErrIDNotFound = errors.New("ID not found")
)

func SubsForChannel(channel string) (result []*ChannelSubscription, err error) {
	err = common.GORM.Where("youtube_channel_id = ?", channel).Find(&result).Error
	return
}

var (
	ErrNoChannel = errors.New("No channel with that id found")
)

func (p *Plugin) AddFeed(guildID, discordChannelID int64, youtubeChannelID, youtubeUsername string, mentionEveryone bool) (*ChannelSubscription, error) {
	sub := &ChannelSubscription{
		GuildID:         discordgo.StrID(guildID),
		ChannelID:       discordgo.StrID(discordChannelID),
		MentionEveryone: mentionEveryone,
	}

	call := p.YTService.Channels.List("snippet")
	if youtubeChannelID != "" {
		call = call.Id(youtubeChannelID)
	} else {
		call = call.ForUsername(youtubeUsername)
	}

	cResp, err := call.Do()
	if err != nil {
		return nil, common.ErrWithCaller(err)
	}

	if len(cResp.Items) < 1 {
		return nil, ErrNoChannel
	}

	sub.YoutubeChannelName = cResp.Items[0].Snippet.Title
	sub.YoutubeChannelID = cResp.Items[0].Id

	err = common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 10)
	if err != nil {
		return nil, err
	}
	defer common.UnlockRedisKey(RedisChannelsLockKey)

	err = common.GORM.Create(sub).Error
	if err != nil {
		return nil, err
	}

	err = p.MaybeAddChannelWatch(false, sub.YoutubeChannelID)
	return sub, err
}

// maybeRemoveChannelWatch checks the channel for subs, if it has none then it removes it from the watchlist in redis.
func (p *Plugin) MaybeRemoveChannelWatch(channel string) {
	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 10)
	if err != nil {
		return
	}
	defer common.UnlockRedisKey(RedisChannelsLockKey)

	var count int
	err = common.GORM.Model(&ChannelSubscription{}).Where("youtube_channel_id = ?", channel).Count(&count).Error
	if err != nil || count > 0 {
		if err != nil {
			logger.WithError(err).WithField("yt_channel", channel).Error("Failed getting sub count")
		}
		return
	}

	err = common.MultipleCmds(
		radix.Cmd(nil, "DEL", KeyLastVidTime(channel)),
		radix.Cmd(nil, "DEL", KeyLastVidID(channel)),
		radix.Cmd(nil, "ZREM", RedisKeyWebSubChannels, channel),
	)

	if err != nil {
		return
	}

	err = p.WebSubUnsubscribe(channel)
	if err != nil {
		logger.WithError(err).Error("Failed unsubscribing to channel ", channel)
	}

	logger.WithField("yt_channel", channel).Info("Removed orphaned youtube channel from subbed channel sorted set")
}

// maybeAddChannelWatch adds a channel watch to redis, if there wasn't one before
func (p *Plugin) MaybeAddChannelWatch(lock bool, channel string) error {
	if lock {
		err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 10)
		if err != nil {
			return common.ErrWithCaller(err)
		}
		defer common.UnlockRedisKey(RedisChannelsLockKey)
	}

	now := time.Now().Unix()

	mn := radix.MaybeNil{}
	err := common.RedisPool.Do(radix.Cmd(&mn, "ZSCORE", RedisKeyWebSubChannels, channel))
	if err != nil {
		return err
	}

	if !mn.Nil {
		// Websub subscription already active, don't do anything more
		return nil
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyLastVidTime(channel), now))
	if err != nil {
		return err
	}

	// Also add websub subscription
	err = p.WebSubSubscribe(channel)
	if err != nil {
		logger.WithError(err).Error("Failed subscribing to channel ", channel)
	}

	logger.WithField("yt_channel", channel).Info("Added new youtube channel watch")
	return nil
}

func (p *Plugin) CheckVideo(videoID string, channelID string) error {
	subs, err := p.getRemoveSubs(channelID)
	if err != nil || len(subs) < 1 {
		return err
	}

	lastVid, lastVidTime, err := p.getLastVidTimes(channelID)
	if err != nil {
		return err
	}

	if lastVid == videoID {
		// the video was already posted and was probably just edited
		return nil
	}

	resp, err := p.YTService.Videos.List("snippet").Id(videoID).Do()
	if err != nil || len(resp.Items) < 1 {
		return err
	}

	item := resp.Items[0]

	if item.Snippet.LiveBroadcastContent != "none" {
		// ignore livestreams for now, might enable them at some point
		return nil
	}

	parsedPublishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
	if err != nil {
		return errors.New("Failed parsing youtube timestamp: " + err.Error() + ": " + item.Snippet.PublishedAt)
	}

	if time.Since(parsedPublishedAt) > time.Hour {
		// just a safeguard against empty lastVidTime's
		return nil
	}

	if lastVidTime.After(parsedPublishedAt) {
		// wasn't a new vid
		return nil
	}

	// This is a new video, post it
	return p.postVideo(subs, parsedPublishedAt, item, channelID)
}

func (p *Plugin) postVideo(subs []*ChannelSubscription, publishedAt time.Time, video *youtube.Video, channelID string) error {
	err := common.MultipleCmds(
		radix.FlatCmd(nil, "SET", KeyLastVidTime(channelID), publishedAt.Unix()),
		radix.FlatCmd(nil, "SET", KeyLastVidID(channelID), video.Id),
	)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		p.sendNewVidMessage(sub.GuildID, sub.ChannelID, video.Snippet.ChannelTitle, video.Id, sub.MentionEveryone)
	}

	return nil
}

func (p *Plugin) getRemoveSubs(channelID string) ([]*ChannelSubscription, error) {
	var subs []*ChannelSubscription
	err := common.GORM.Where("youtube_channel_id = ?", channelID).Find(&subs).Error
	if err != nil {
		return subs, err
	}

	if len(subs) < 1 {
		time.AfterFunc(time.Second*10, func() {
			p.MaybeRemoveChannelWatch(channelID)
		})
		return subs, nil
	}

	return subs, nil
}

func (p *Plugin) getLastVidTimes(channelID string) (lastVid string, lastVidTime time.Time, err error) {
	// Find the last video time for this channel
	var unixSeconds int64
	err = common.RedisPool.Do(radix.Cmd(&unixSeconds, "GET", KeyLastVidTime(channelID)))

	var lastProcessedVidTime time.Time
	if err != nil || unixSeconds == 0 {
		lastProcessedVidTime = time.Time{}
	} else {
		lastProcessedVidTime = time.Unix(unixSeconds, 0)
	}

	var lastVidID string
	err = common.RedisPool.Do(radix.Cmd(&lastVidID, "GET", KeyLastVidID(channelID)))
	return lastVidID, lastProcessedVidTime, err
}
