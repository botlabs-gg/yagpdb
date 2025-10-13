package youtube

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/youtube/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	WebSubCheckInterval = time.Second * 10
)

func (p *Plugin) StartFeed() {
	p.Stop = make(chan *sync.WaitGroup)
	go p.runWebsubChecker()
	go p.autoSyncWebsubs()
	go p.deleteOldVideos()
}

func (p *Plugin) StopFeed(wg *sync.WaitGroup) {
	if p.Stop != nil {
		p.Stop <- wg
	} else {
		wg.Done()
	}
}

func (p *Plugin) SetupClient() error {
	yt, err := youtube.NewService(context.Background(), option.WithScopes(youtube.YoutubeScope))
	if err != nil {
		return common.ErrWithCaller(err)
	}
	p.YTService = yt
	return nil
}

func (p *Plugin) deleteOldVideos() {
	ticker := time.NewTicker(time.Minute * 1)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			var expiring int64
			videoCacheDays := max(confYoutubeVideoCacheDays.GetInt(), 1)
			common.RedisPool.Do(radix.FlatCmd(&expiring, "ZREMRANGEBYSCORE", RedisKeyPublishedVideoList, "-inf", time.Now().AddDate(0, 0, -1*videoCacheDays).Unix()))
			logger.Infof("Removed %d old videos", expiring)
		}
	}
}

func (p *Plugin) autoSyncWebsubs() {
	// force syncs all websubs from db every 24 hours in case of outages or missed updates
	ticker := time.NewTicker(time.Hour * 1)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			p.syncWebSubs()
		}
	}
}

// keeps the subscriptions up to date by updating the ones soon to be expiring
func (p *Plugin) runWebsubChecker() {
	// If youtube feed is restarting and the previous run was stopped, we need to unlock the lock
	common.UnlockRedisKey(RedisChannelsLockKey)
	p.syncWebSubs()
	ticker := time.NewTicker(WebSubCheckInterval)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			p.checkExpiringWebsubs()
		}
	}
}

// Concurrently runs fn for each channel with at most workerCount in flight
func (p *Plugin) processChannelsConcurrently(channels []string, workerCount int, fn func(string)) {
	if workerCount < 1 {
		workerCount = 1
	}
	jobs := make(chan string, workerCount*2)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Go(func() {
			for ch := range jobs {
				fn(ch)
			}
		})
	}
	for _, ch := range channels {
		jobs <- ch
	}
	close(jobs)
	wg.Wait()
}

func (p *Plugin) webSubSubscribeWithRetry(channel string) {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := p.WebSubSubscribe(channel)
		if err == nil {
			return
		}
		if attempt == maxRetries {
			logger.WithError(err).WithField("yt_channel", channel).Error("websub subscribe failed after retries")
			return
		}
		time.Sleep(time.Second * time.Duration(1<<attempt))
	}
}

func (p *Plugin) checkExpiringWebsubs() {
	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 10)
	if err != nil {
		logger.WithError(err).Error("Failed locking channels lock")
		return
	}

	maxScore := time.Now().Unix()
	var expiring []string
	err = common.RedisPool.Do(radix.FlatCmd(&expiring, "ZRANGEBYSCORE", RedisKeyWebSubChannels, "-inf", maxScore))
	if err != nil {
		logger.WithError(err).Error("Failed checking websubs")
		common.UnlockRedisKey(RedisChannelsLockKey)
		return
	}

	// Unlock early; subscribing does not need to hold the redis lock
	common.UnlockRedisKey(RedisChannelsLockKey)

	batchSize := confResubBatchSize.GetInt()
	totalExpiring := len(expiring)
	logger.Infof("Found %d expiring subs", totalExpiring)
	channelChunks := make([][]string, 0)
	for i := 0; i < totalExpiring; i += batchSize {
		end := min(i+batchSize, totalExpiring)
		channelChunks = append(channelChunks, expiring[i:end])
	}
	for i, chunk := range channelChunks {
		logger.Infof("Processing chunk %d of %d for expiring subs", i, len(channelChunks))
		p.processChannelsConcurrently(chunk, batchSize, func(ch string) { p.webSubSubscribeWithRetry(ch) })
	}
}

func (p *Plugin) syncWebSubs() {
	var activeChannels []string
	err := common.SQLX.Select(&activeChannels, "SELECT DISTINCT(youtube_channel_id) FROM youtube_channel_subscriptions;")
	if err != nil {
		logger.WithError(err).Error("Failed syncing websubs, failed retrieving subbed channels")
		return
	}

	// Lock only while reading from Redis to determine which channels need resubscribe
	if err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5000); err != nil {
		logger.WithError(err).Error("Failed locking channels lock")
		return
	}

	var expiring []string
	_ = common.RedisPool.Do(radix.WithConn(RedisKeyWebSubChannels, func(client radix.Conn) error {
		logger.Infof("Found %d youtube channels", len(activeChannels))
		for _, channel := range activeChannels {
			var mn int64
			client.Do(radix.Cmd(&mn, "ZSCORE", RedisKeyWebSubChannels, channel))
			if mn < time.Now().Unix() {
				expiring = append(expiring, channel)
			}
		}
		return nil
	}))

	common.UnlockRedisKey(RedisChannelsLockKey)
	batchSize := confResubBatchSize.GetInt()
	channelChunks := make([][]string, 0)
	totalExpiring := len(expiring)
	logger.Infof("Found %d expiring subs to youtube", totalExpiring)
	for i := 0; i < totalExpiring; i += batchSize {
		end := min(i+batchSize, totalExpiring)
		channelChunks = append(channelChunks, activeChannels[i:end])
	}
	for i, chunk := range channelChunks {
		logger.Infof("Processing chunk %d of %d for expiring subs", i, len(channelChunks))
		p.processChannelsConcurrently(chunk, batchSize, func(ch string) { p.webSubSubscribeWithRetry(ch) })
	}
}

func (p *Plugin) sendNewVidMessage(sub *models.YoutubeChannelSubscription, video *youtube.Video) {
	parsedChannel, _ := strconv.ParseInt(sub.ChannelID, 10, 64)
	parsedGuild, _ := strconv.ParseInt(sub.GuildID, 10, 64)
	videoUrl := "https://www.youtube.com/watch?v=" + video.Id

	var content string
	switch video.Snippet.LiveBroadcastContent {
	case "live":
		content = fmt.Sprintf("**%s** started a livestream now!\n%s", video.Snippet.ChannelTitle, videoUrl)
	case "upcoming":
		content = fmt.Sprintf("**%s** is going to be live soon!\n%s", video.Snippet.ChannelTitle, videoUrl)
	case "none":
		content = fmt.Sprintf("**%s** uploaded a new youtube video!\n%s", video.Snippet.ChannelTitle, videoUrl)
	default:
		return
	}

	parseMentions := []discordgo.AllowedMentionType{}
	announcement, err := models.FindYoutubeAnnouncementG(context.Background(), parsedGuild)
	hasCustomAnnouncement := true
	if err != nil {
		hasCustomAnnouncement = false
		if err == sql.ErrNoRows {
			logger.WithError(err).Debugf("Custom announcement doesn't exist for guild_id %d", parsedGuild)
		} else {
			logger.WithError(err).Errorf("Failed fetching custom announcement for guild_id %d", parsedGuild)
		}
	}

	if hasCustomAnnouncement && announcement.Enabled && len(announcement.Message) > 0 {
		pubsub.Publish("custom_youtube_announcement", parsedGuild, CustomYoutubeAnnouncement{
			GuildID:      parsedGuild,
			Subscription: *sub,
			Video:        *video,
		})
		return
	} else if sub.MentionEveryone {
		content = "Hey @everyone " + content
		parseMentions = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeEveryone}
	} else if len(sub.MentionRoles) > 0 {
		mentions := "Hey"
		for _, roleId := range sub.MentionRoles {
			mentions += fmt.Sprintf(" <@&%d>", roleId)
		}
		content = mentions + " " + content
		parseMentions = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles}
	}

	go analytics.RecordActiveUnit(parsedGuild, p, "posted_youtube_message")
	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "youtube"}).Inc()
	mqueue.QueueMessage(&mqueue.QueuedElement{
		GuildID:             parsedGuild,
		ChannelID:           parsedChannel,
		Source:              "youtube",
		SourceItemID:        "",
		MessageStr:          content,
		Priority:            2,
		PublishAnnouncement: false,
		AllowedMentions: discordgo.AllowedMentions{
			Parse: parseMentions,
		},
	})
}

var (
	ErrIDNotFound = errors.New("ID not found")
)

var (
	ErrNoChannel              = errors.New("no channel with that id found")
	ErrMaxCustomMessageLength = errors.New("max length of custom message can be 500 chars")
)

var listParts = []string{"snippet"}

type ytChannelID interface {
	getChannelList(p *Plugin, list *youtube.ChannelsListCall) (cResp *youtube.ChannelListResponse, err error)
}

type videoID string

func (v videoID) getChannelList(p *Plugin, list *youtube.ChannelsListCall) (cResp *youtube.ChannelListResponse, err error) {

	videoListCall := p.YTService.Videos.List(listParts)
	vResp, err := videoListCall.Id(string(v)).MaxResults(1).Do()
	if err != nil {
		return nil, common.ErrWithCaller(err)
	} else if len(vResp.Items) < 1 {
		return nil, errors.New("video not found")
	}
	cResp, err = list.Id(vResp.Items[0].Snippet.ChannelId).Do()
	return cResp, common.ErrWithCaller(err)

}

type channelID string

func (c channelID) getChannelList(p *Plugin, list *youtube.ChannelsListCall) (cResp *youtube.ChannelListResponse, err error) {
	cResp, err = list.Id(string(c)).Do()
	return cResp, common.ErrWithCaller(err)
}

type userID string

func (u userID) getChannelList(p *Plugin, list *youtube.ChannelsListCall) (cResp *youtube.ChannelListResponse, err error) {
	cResp, err = list.ForUsername(string(u)).Do()
	return cResp, common.ErrWithCaller(err)
}

type searchChannelID string

func (s searchChannelID) getChannelList(p *Plugin, list *youtube.ChannelsListCall) (cResp *youtube.ChannelListResponse, err error) {
	q := string(s)
	searchListCall := p.YTService.Search.List(listParts)
	sResp, err := searchListCall.Q(q).Type("channel").MaxResults(1).Do()
	if err != nil {
		return nil, common.ErrWithCaller(err)
	} else if len(sResp.Items) < 1 {
		return nil, ErrNoChannel
	}
	cResp, err = list.Id(sResp.Items[0].Id.ChannelId).Do()
	return cResp, common.ErrWithCaller(err)
}

type playlistID string

func (pl playlistID) getChannelList(p *Plugin, list *youtube.ChannelsListCall) (cResp *youtube.ChannelListResponse, err error) {
	id := string(pl)
	playlistListCall := p.YTService.Playlists.List(listParts)
	pResp, err := playlistListCall.Id(id).MaxResults(1).Do()
	if err != nil {
		return nil, common.ErrWithCaller(err)
	} else if len(pResp.Items) < 1 {
		return nil, ErrNoChannel
	}
	cResp, err = list.Id(pResp.Items[0].Snippet.ChannelId).Do()
	return cResp, common.ErrWithCaller(err)
}

func (p *Plugin) parseYtUrl(channelUrl *url.URL) (id ytChannelID, err error) {
	// First set of URL types should only have one segment,
	// so trimming leading forward slash simplifies following operations
	path := strings.TrimPrefix(channelUrl.Path, "/")
	if path == "" {
		return nil, errors.New("url must feature a path to identify a YouTube channel")
	}

	host := channelUrl.Host
	if strings.HasSuffix(host, "youtu.be") {
		return p.parseYtVideoID(path)
	} else if !strings.HasSuffix(host, "youtube.com") {
		return nil, fmt.Errorf("%q is not a valid youtube domain", host)
	}

	if strings.HasPrefix(path, "watch") {
		// `v` key-value pair should identify the video ID
		// in URLs with a `watch` segment.
		val := channelUrl.Query().Get("v")
		return p.parseYtVideoID(val)
	} else if strings.HasPrefix(path, "playlist") {
		val := channelUrl.Query().Get("list")
		if ytPlaylistIDRegex.MatchString(val) {
			return playlistID(val), nil
		} else {
			return nil, fmt.Errorf("%q is not a valid playlist ID", val)
		}
	}

	// As each section of a channel has a second part of the URL,
	// we need to split the path now to accommodate for this.
	//
	// Due to the earlier check, we know `pathSegements` will have
	// at least one segment, or we would've already returned an error,
	// but we still only need the first segment for this check.
	pathSegments := strings.Split(path, "/")
	first := pathSegments[0]

	// Prefix check allows method to provide a more helpful error message,
	// when attempting to parse an invalid handle URL.
	if strings.HasPrefix(first, "@") {
		if ytHandleRegex.MatchString(first) {
			return searchChannelID(first), nil
		} else {
			return nil, fmt.Errorf("%q is not a valid youtube handle", path)
		}
	}

	// Similarly to handle URLs, other types of channel URL
	// may have more than two segments.
	if len(pathSegments) < 2 {
		return nil, fmt.Errorf("%q is not a valid path", path)
	}

	// From now on, all parsed URLs will be based
	// on the second segment of their path.
	second := pathSegments[1]

	switch first {
	case "shorts", "live":
		return p.parseYtVideoID(second)
	case "channel":
		if ytChannelIDRegex.MatchString(second) {
			return channelID(second), nil
		} else {
			return nil, fmt.Errorf("%q is not a valid youtube channel id", id)
		}
	case "c":
		return searchChannelID(second), nil
	case "user":
		return userID(second), nil
	default:
		return nil, fmt.Errorf("%q is not a valid path", path)
	}
}

func (p *Plugin) parseYtVideoID(parse string) (id ytChannelID, err error) {
	if ytVideoIDRegex.MatchString(parse) {
		return videoID(parse), nil
	} else {
		return nil, fmt.Errorf("%q is not a valid youtube video id", parse)
	}
}

func (p *Plugin) AddFeed(guildID, discordChannelID int64, ytChannel *youtube.Channel, mentionEveryone bool, publishLivestream bool, publishShorts bool, mentionRoles []int64) (*models.YoutubeChannelSubscription, error) {
	if mentionEveryone && len(mentionRoles) > 0 {
		mentionRoles = make([]int64, 0)
	}

	sub := &models.YoutubeChannelSubscription{
		GuildID:           discordgo.StrID(guildID),
		ChannelID:         discordgo.StrID(discordChannelID),
		MentionEveryone:   mentionEveryone,
		MentionRoles:      mentionRoles,
		PublishLivestream: publishLivestream,
		PublishShorts:     publishShorts,
		Enabled:           true,
	}

	sub.YoutubeChannelName = ytChannel.Snippet.Title
	sub.YoutubeChannelID = ytChannel.Id

	err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 10)
	if err != nil {
		return nil, err
	}
	defer common.UnlockRedisKey(RedisChannelsLockKey)

	err = sub.InsertG(context.Background(), boil.Infer())
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

	count, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.YoutubeChannelID.EQ(channel),
	).CountG(context.Background())
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
	logger.Info("Added websub")
	err = p.WebSubSubscribe(channel)
	if err != nil {
		logger.WithError(err).Error("Failed subscribing to channel ", channel)
	}

	logger.WithField("yt_channel", channel).Info("Added new youtube channel watch")
	return nil
}

func (p *Plugin) CheckVideo(parsedVideo XMLFeed) error {
	if parsedVideo.VideoId == "" || parsedVideo.ChannelID == "" {
		return nil
	}

	videoID := parsedVideo.VideoId
	channelID := parsedVideo.ChannelID
	logger.Infof("Checking new video request with videoID %s and channelID %s ", videoID, channelID)

	parsedPublishedTime, err := time.Parse(time.RFC3339, parsedVideo.Published)
	if err != nil {
		return errors.New("Failed parsing youtube timestamp: " + err.Error() + ": " + parsedVideo.Published)
	}

	videoCacheDays := max(confYoutubeVideoCacheDays.GetInt(), 1)
	if time.Since(parsedPublishedTime) > time.Hour*24*time.Duration(videoCacheDays) {
		// don't post videos older than videoCacheDays
		logger.Infof("Skipped Stale video (published more than %d days ago) for youtube channel %s: video_id: %s", videoCacheDays, channelID, videoID)
		return nil
	}

	subs, err := p.getRemoveSubs(channelID)
	if err != nil || len(subs) < 1 {
		return err
	}

	mn := radix.MaybeNil{}
	common.RedisPool.Do(radix.Cmd(&mn, "ZSCORE", RedisKeyPublishedVideoList, videoID))
	if !mn.Nil {
		// video was already published, maybe just an update on it?
		logger.Infof("Skipped Already Published video for youtube channel %s: video_id: %s", channelID, videoID)
		return nil
	}

	resp, err := p.YTService.Videos.List([]string{"snippet", "contentDetails"}).Id(videoID).Do()
	if err != nil || len(resp.Items) < 1 {
		return err
	}

	item := resp.Items[0]

	// This is a new video, post it
	return p.postVideo(subs, parsedPublishedTime, item, channelID)
}

func (p *Plugin) isShortsVideo(video *youtube.Video) bool {
	if video.Snippet.LiveBroadcastContent == "live" {
		return false
	}
	if video.ContentDetails == nil {
		logger.Errorf("contentDetails was nil for youtube video id %s, isLiveStream? %s", video.Id, video.Snippet.LiveBroadcastContent)
		return false
	}
	videoDurationString := strings.ToLower(strings.TrimPrefix(video.ContentDetails.Duration, "PT"))
	videoDuration, err := common.ParseDuration(videoDurationString)
	if err != nil {
		return false
	}
	// videos below 3 minutes can be shorts
	// the 10 second is a buffer as youtube is stupid sometimes
	if videoDuration >= (time.Minute*3 + time.Second*10) {
		return false
	}
	isShort := p.isShortsRedirect(video.Id)
	logger.Infof("Video %s is a shorts video?: %t", video.Id, isShort)
	return isShort
}

func (p *Plugin) isShortsRedirect(videoId string) bool {
	shortsUrl := fmt.Sprintf("https://www.youtube.com/shorts/%s?ucbcb=1", videoId)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("HEAD", shortsUrl, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to make youtube shorts request")
		return false
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.79 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Error("Failed to make youtube shorts request")
		return false
	}

	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (p *Plugin) postVideo(subs models.YoutubeChannelSubscriptionSlice, publishedAt time.Time, video *youtube.Video, channelID string) error {
	// add video to list of published videos
	err := common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", RedisKeyPublishedVideoList, publishedAt.Unix(), video.Id))
	if err != nil {
		return err
	}

	contentType := video.Snippet.LiveBroadcastContent
	logger.Infof("Got a new video for channel %s with videoid %s, of type %s and publishing to %d subscriptions", channelID, video.Id, contentType, len(subs))

	isLivestream := contentType == "live"
	isUpcoming := contentType == "upcoming"
	isShortsCheckDone := false
	isShorts := false

	for _, sub := range subs {
		if sub.Enabled {
			if (isLivestream || isUpcoming) && !sub.PublishLivestream {
				continue
			}

			//no need to check for shorts for a livestream
			if !(isLivestream || isUpcoming) && !sub.PublishShorts {
				//check if a video is a short only when seeing the first shorts disabled subscription
				//and cache in "isShorts" to reduce requests to youtube to check for shorts.
				if !isShortsCheckDone {
					isShorts = p.isShortsVideo(video)
					isShortsCheckDone = true
				}

				if isShorts {
					continue
				}
			}
			p.sendNewVidMessage(sub, video)
		}
	}

	return nil
}

func (p *Plugin) getRemoveSubs(channelID string) (models.YoutubeChannelSubscriptionSlice, error) {
	subs, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.YoutubeChannelID.EQ(channelID),
	).AllG(context.Background())
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
