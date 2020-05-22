package youtube

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
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
	go p.runWebsubChecker()
	p.runFeed()
}

func (p *Plugin) StopFeed(wg *sync.WaitGroup) {

	if p.Stop != nil {
		wg.Add(1)
		p.Stop <- wg
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

func (p *Plugin) runFeed() {

	ticker := time.NewTicker(PollInterval)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			// now := time.Now()
			err := p.checkChannels()
			if err != nil {
				logger.WithError(err).Error("Failed checking youtube channels")
			}

			// logger.Info("Took", time.Since(now), "to check youtube feeds")
		}
	}
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
	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5)
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
	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5000)
	if err != nil {
		logger.WithError(err).Error("Failed locking channels lock")
		return
	}
	defer common.UnlockRedisKey(RedisChannelsLockKey)

	var activeChannels []string
	err = common.RedisPool.Do(radix.Cmd(&activeChannels, "ZRANGEBYSCORE", "youtube_subbed_channels", "-inf", "+inf"))
	if err != nil {
		logger.WithError(err).Error("Failed syncing websubs, failed retrieving subbed channels")
		return
	}

	common.RedisPool.Do(radix.WithConn(RedisKeyWebSubChannels, func(client radix.Conn) error {
		for _, channel := range activeChannels {
			mn := radix.MaybeNil{}
			client.Do(radix.Cmd(&mn, "ZSCORE", RedisKeyWebSubChannels, channel))
			if mn.Nil {
				// Not added
				err := p.WebSubSubscribe(channel)
				if err != nil {
					logger.WithError(err).WithField("yt_channel", channel).Error("Failed subscribing to channel")
				}

				time.Sleep(time.Second)
			}
		}

		return nil
	}))
}

func (p *Plugin) checkChannels() error {
	var channels []string
	err := common.RedisPool.Do(radix.FlatCmd(&channels, "ZRANGE", "youtube_subbed_channels", 0, MaxChannelsPerPoll))
	if err != nil {
		return err
	}

	for _, channel := range channels {
		err = p.checkChannel(channel)
		if err != nil {
			if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
				// This channel has been deleted
				logger.WithError(err).WithField("yt_channel", channel).Warn("Removing non existant youtube channel")
				p.removeAllSubsForChannel(channel)
			} else if err == ErrIDNotFound {
				// This can happen if the channel was terminated because it broke the terms for example, just remove all references to it
				logger.WithField("channel", channel).Info("Removing youtube feed to channel without playlist")
				p.removeAllSubsForChannel(channel)
			} else {
				logger.WithError(err).WithField("yt_channel", channel).Error("Failed checking youtube channel")
			}
		}
	}

	return nil
}

func (p *Plugin) removeAllSubsForChannel(channel string) {
	err := common.GORM.Where("youtube_channel_id = ?", channel).Delete(ChannelSubscription{}).Error
	if err != nil {
		logger.WithError(err).WithField("yt_channel", channel).Error("failed removing channel")
	}
	go p.MaybeRemoveChannelWatch(channel)
}

func (p *Plugin) checkChannel(channel string) error {
	now := time.Now()

	var subs []*ChannelSubscription
	err := common.GORM.Where("youtube_channel_id = ?", channel).Find(&subs).Error
	if err != nil {
		return err
	}

	if len(subs) < 1 {
		time.AfterFunc(time.Second*10, func() {
			p.MaybeRemoveChannelWatch(channel)
		})
		return nil
	}

	playlistID, err := p.PlaylistID(channel)
	if err != nil {
		return err
	}

	// Find the last video time for this channel
	var unixSeconds int64
	err = common.RedisPool.Do(radix.Cmd(&unixSeconds, "GET", KeyLastVidTime(channel)))

	var lastProcessedVidTime time.Time
	if err != nil || unixSeconds == 0 {
		if err != nil {
			logger.WithError(err).Error("Failed retrieving last processed vid time, falling back to this time")
		}

		lastProcessedVidTime = time.Now()
		common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyLastVidTime(channel), lastProcessedVidTime.Unix()))
	} else {
		lastProcessedVidTime = time.Unix(unixSeconds, 0)
	}

	var lastVidID string
	common.RedisPool.Do(radix.Cmd(&lastVidID, "GET", KeyLastVidID(channel)))

	// latestVid is used to set the last vid id and time
	var latestVid *youtube.PlaylistItem
	first := true
	nextPage := ""
	for {
		call := p.YTService.PlaylistItems.List("snippet").PlaylistId(playlistID).MaxResults(50)
		if nextPage != "" {
			call = call.PageToken(nextPage)
		}

		resp, err := call.Do()
		if err != nil {

			return err
		}
		if first {
			if len(resp.Items) > 0 {
				latestVid = resp.Items[0]
			}
			first = false
		}

		lv, done, err := p.handlePlaylistItemsResponse(resp, subs, lastProcessedVidTime, lastVidID)
		if err != nil {
			return err
		}
		if lv != nil {
			// compare lv, the latest video in the response, and latestVid, the current latest video tracked for this channel
			parsedPublishedAtLv, _ := time.Parse(time.RFC3339, lv.Snippet.PublishedAt)
			parsedPublishedOld, err := time.Parse(time.RFC3339, latestVid.Snippet.PublishedAt)
			if err != nil {
				logger.WithError(err).WithField("vid", latestVid.Id).Error("Failed parsing publishedat")
			} else {
				if parsedPublishedAtLv.After(parsedPublishedOld) {
					latestVid = lv
				}
			}
		}

		if done {
			break
		}

		logger.Debug("next", resp.NextPageToken)
		if resp.NextPageToken == "" {
			break // Reached end
		}
		nextPage = resp.NextPageToken
	}

	common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", "youtube_subbed_channels", now.Unix(), channel))

	// Update the last vid id and time if needed
	if latestVid != nil {
		parsedTime, _ := time.Parse(time.RFC3339, latestVid.Snippet.PublishedAt)
		if !lastProcessedVidTime.After(parsedTime) && (latestVid.Id != lastVidID || parsedTime.Unix() != lastProcessedVidTime.Unix()) {
			common.MultipleCmds(
				radix.FlatCmd(nil, "SET", KeyLastVidTime(channel), parsedTime.Unix()),
				radix.FlatCmd(nil, "SET", KeyLastVidID(channel), latestVid.Id),
			)
		}
	}

	return nil
}

func (p *Plugin) handlePlaylistItemsResponse(resp *youtube.PlaylistItemListResponse, subs []*ChannelSubscription, lastProcessedVidTime time.Time, lastVidID string) (latest *youtube.PlaylistItem, complete bool, err error) {

	var latestTime time.Time

	for _, item := range resp.Items {

		parsedPublishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if err != nil {
			logger.WithError(err).Error("Failed parsing video time")
			continue
		}

		// Video is published before the latest video we checked, mark as complete and do not post messages for
		if !parsedPublishedAt.After(lastProcessedVidTime) || item.Id == lastVidID {
			complete = true
			continue
		}

		logger.Info("Found youtube upload: ", item.Snippet.ChannelTitle, ": ", item.Snippet.Title, " : ", parsedPublishedAt.Format(time.RFC3339))

		// This is the new latest video
		if parsedPublishedAt.After(latestTime) {
			latestTime = parsedPublishedAt
			latest = item
		}

		for _, sub := range subs {
			go p.sendNewVidMessage(sub.GuildID, sub.ChannelID, item.Snippet.ChannelTitle, item.Snippet.ResourceId.VideoId, sub.MentionEveryone)
		}

		feeds.MetricPostedMessages.With(prometheus.Labels{"source": "youtube"}).Add(float64(len(subs)))
	}

	return
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

func (p *Plugin) PlaylistID(channelID string) (string, error) {

	var entry YoutubePlaylistID
	err := common.GORM.Where("channel_id = ?", channelID).First(&entry).Error
	if err == nil {
		return entry.PlaylistID, nil
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}

	cResp, err := p.YTService.Channels.List("contentDetails").Id(channelID).Do()
	if err != nil {
		return "", err
	}

	if len(cResp.Items) < 1 {
		return "", ErrIDNotFound
	}

	id := cResp.Items[0].ContentDetails.RelatedPlaylists.Uploads

	entry.ChannelID = channelID
	entry.PlaylistID = id

	common.GORM.Create(&entry)

	return id, nil
}

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

	err = common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5)
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
	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5)
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
		radix.Cmd(nil, "ZREM", "youtube_subbed_channels", channel),
		radix.Cmd(nil, "DEL", KeyLastVidTime(channel)),
		radix.Cmd(nil, "DEL", KeyLastVidID(channel)),
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
		err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5)
		if err != nil {
			return common.ErrWithCaller(err)
		}
		defer common.UnlockRedisKey(RedisChannelsLockKey)
	}

	now := time.Now().Unix()

	mn := radix.MaybeNil{}
	err := common.RedisPool.Do(radix.Cmd(&mn, "ZSCORE", "youtube_subbed_channels", channel))
	if err != nil {
		return common.ErrWithCaller(err)
	}

	if !mn.Nil {
		// already added before, don't need to do anything
		logger.Debug("Not nil reply")
		return nil
	}

	common.MultipleCmds(
		radix.FlatCmd(nil, "ZADD", "youtube_subbed_channels", now, channel),
		radix.FlatCmd(nil, "SET", KeyLastVidTime(channel), now),
	)

	// Also add websub subscription
	err = p.WebSubSubscribe(channel)
	if err != nil {
		logger.WithError(err).Error("Failed subscribing to channel ", channel)
	}

	logger.WithField("yt_channel", channel).Info("Added new youtube channel watch")
	return nil
}
