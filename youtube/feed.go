package youtube

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
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
	yt, err := youtube.NewService(context.Background(), option.WithScopes(youtube.YoutubeScope))
	if err != nil {
		return common.ErrWithCaller(err)
	}
	p.YTService = yt
	return nil
}

// keeps the subscriptions up to date by updating the ones soon to be expiring
func (p *Plugin) runWebsubChecker() {
	go p.syncWebSubs()

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

	totalExpiring := len(expiring)
	batchSize := confResubBatchSize.GetInt()
	logger.Infof("Found %d expiring subs", totalExpiring)
	expiringChunks := make([][]string, 0)
	for i := 0; i < totalExpiring; i += batchSize {
		end := i + batchSize
		if end > totalExpiring {
			end = totalExpiring
		}
		expiringChunks = append(expiringChunks, expiring[i:end])
	}
	for index, chunk := range expiringChunks {
		logger.Infof("Processing chunk %d of %d for %d expiring youtube subs", index+1, len(expiringChunks), totalExpiring)
		for _, sub := range chunk {
			go p.WebSubSubscribe(sub)
		}
		// sleep for a second before processing next chunk
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
		if !locked {
			err := common.BlockingLockRedisKey(RedisChannelsLockKey, 0, 5000)
			if err != nil {
				logger.WithError(err).Error("Failed locking channels lock")
				return err
			}
			locked = true
		}

		totalChannels := len(activeChannels)
		batchSize := confResubBatchSize.GetInt()
		logger.Infof("Found %d youtube channels", totalChannels)
		channelChunks := make([][]string, 0)
		for i := 0; i < totalChannels; i += batchSize {
			end := i + batchSize
			if end > totalChannels {
				end = totalChannels
			}
			channelChunks = append(channelChunks, activeChannels[i:end])
		}
		for index, chunk := range channelChunks {
			logger.Infof("Processing chunk %d of %d for %d youtube channels", index+1, len(channelChunks), totalChannels)
			for _, channel := range chunk {
				mn := radix.MaybeNil{}
				client.Do(radix.Cmd(&mn, "ZSCORE", RedisKeyWebSubChannels, channel))
				if mn.Nil {
					// Channel not added to redis, resubscribe and add to redis
					go p.WebSubSubscribe(channel)
				}
			}
		}
		if locked {
			common.UnlockRedisKey(RedisChannelsLockKey)
		}
		return nil
	}))
}

func (p *Plugin) sendNewVidMessage(sub *ChannelSubscription, video *youtube.Video) {
	parsedChannel, _ := strconv.ParseInt(sub.ChannelID, 10, 64)
	parsedGuild, _ := strconv.ParseInt(sub.GuildID, 10, 64)
	videoUrl := "https://www.youtube.com/watch?v=" + video.Id
	var announcement YoutubeAnnouncements
	err := common.GORM.Model(&YoutubeAnnouncements{}).Where("guild_id = ?", parsedGuild).First(&announcement).Error
	if err != nil {
		logger.WithError(err).Errorf("Failed fetching youtube message from db for guild_id %d", parsedGuild)
		return
	}

	var content string
	switch video.Snippet.LiveBroadcastContent {
	case "live":
		content = fmt.Sprintf("**%s** started a livestream now!\n%s", video.Snippet.ChannelTitle, videoUrl)
	case "none":
		content = fmt.Sprintf("**%s** uploaded a new youtube video!\n%s", video.Snippet.ChannelTitle, videoUrl)
	default:
		return
	}
	parseMentions := []discordgo.AllowedMentionType{}
	if announcement.Enabled && len(announcement.Message) > 0 {
		guildState := bot.State.GetGuild(parsedGuild)
		if guildState == nil {
			logger.Errorf("guild_id %d not found in state for youtube feed", parsedGuild)
			return
		}
		channelState := guildState.GetChannel(parsedChannel)
		if channelState == nil {
			logger.Errorf("channel_id %d for guild_id %d not found in state for youtube feed", parsedChannel, parsedGuild)
			return
		}
		ctx := templates.NewContext(guildState, channelState, nil)
		ctx.Data["URL"] = videoUrl
		ctx.Data["ChannelName"] = sub.YoutubeChannelName
		ctx.Data["VideoID"] = video.Id
		ctx.Data["VideoTitle"] = video.Snippet.Title
		ctx.Data["VideoThumbnail"] = video.Snippet.Thumbnails.Maxres.Url
		ctx.Data["VideoDescription"] = video.Snippet.Description
		ctx.Data["ChannelID"] = sub.ChannelID
		ctx.Data["IsLiveStream"] = video.Snippet.LiveBroadcastContent == "live"
		content, err = ctx.Execute(announcement.Message)
		if err != nil {
			logger.WithError(err).WithField("guild", parsedGuild).Warn("Announcement parsing failed")
			return
		}
		if content == "" {
			return
		}
	} else {
		if sub.MentionEveryone {
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
	}

	go analytics.RecordActiveUnit(parsedGuild, p, "posted_youtube_message")
	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "youtube"}).Inc()
	mqueue.QueueMessage(&mqueue.QueuedElement{
		GuildID:      parsedGuild,
		ChannelID:    parsedChannel,
		Source:       "youtube",
		SourceItemID: "",
		MessageStr:   content,
		Priority:     2,
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
	ErrNoChannel              = errors.New("no channel with that id found")
	ErrMaxCustomMessageLength = errors.New("max length of custom message can be 500 chars")
)

func (p *Plugin) parseYtUrl(url string) (t ytUrlType, id string, err error) {
	if ytVideoUrlRegex.MatchString(url) {
		capturingGroups := ytVideoUrlRegex.FindAllStringSubmatch(url, -1)
		if len(capturingGroups) > 0 && len(capturingGroups[0]) > 0 && len(capturingGroups[0][4]) > 0 {
			return ytUrlTypeVideo, capturingGroups[0][4], nil
		}
	} else if ytChannelUrlRegex.MatchString(url) {
		capturingGroups := ytChannelUrlRegex.FindAllStringSubmatch(url, -1)
		if len(capturingGroups) > 0 && len(capturingGroups[0]) > 0 && len(capturingGroups[0][5]) > 0 {
			return ytUrlTypeChannel, capturingGroups[0][5], nil
		}
	} else if ytCustomUrlRegex.MatchString(url) {
		capturingGroups := ytCustomUrlRegex.FindAllStringSubmatch(url, -1)
		if len(capturingGroups) > 0 && len(capturingGroups[0]) > 0 && len(capturingGroups[0][5]) > 0 {
			return ytUrlTypeCustom, capturingGroups[0][5], nil
		}
	} else if ytUserUrlRegex.MatchString(url) {
		capturingGroups := ytUserUrlRegex.FindAllStringSubmatch(url, -1)
		if len(capturingGroups) > 0 && len(capturingGroups[0]) > 0 && len(capturingGroups[0][5]) > 0 {
			return ytUrlTypeUser, capturingGroups[0][5], nil
		}
	} else if ytHandleUrlRegex.MatchString(url) {
		capturingGroups := ytHandleUrlRegex.FindAllStringSubmatch(url, -1)
		if len(capturingGroups) > 0 && len(capturingGroups[0]) > 0 && len(capturingGroups[0][5]) > 0 {
			return ytUrlTypeHandle, capturingGroups[0][5], nil
		}
	}
	return ytUrlTypeInvalid, "", errors.New("invalid or incomplete url")
}

func (p *Plugin) getYtChannel(url string) (channel *youtube.Channel, err error) {
	urlType, id, err := p.parseYtUrl(url)
	if err != nil {
		return nil, err
	}
	var cResp *youtube.ChannelListResponse
	channelListCall := p.YTService.Channels.List([]string{"snippet"})

	switch urlType {
	case ytUrlTypeChannel:
		channelListCall = channelListCall.Id(id)
	case ytUrlTypeUser:
		channelListCall = channelListCall.ForUsername(id)
	case ytUrlTypeCustom:
	case ytUrlTypeHandle:
		searchListCall := p.YTService.Search.List([]string{"snippet"})
		searchListCall = searchListCall.Q(id).Type("channel")
		sResp, err := searchListCall.Do()
		if err != nil {
			return nil, common.ErrWithCaller(err)
		}
		if len(sResp.Items) < 1 {
			return nil, ErrNoChannel
		}
		channelListCall = channelListCall.Id(sResp.Items[0].Id.ChannelId)
	case ytUrlTypeVideo:
		searchListCall := p.YTService.Search.List([]string{"snippet"})
		searchListCall = searchListCall.Q(id).Type("video")
		sResp, err := searchListCall.Do()
		if err != nil {
			return nil, common.ErrWithCaller(err)
		}
		if len(sResp.Items) < 1 {
			return nil, ErrNoChannel
		}
		channelListCall = channelListCall.Id(sResp.Items[0].Snippet.ChannelId)
	default:
		return nil, common.ErrWithCaller(errors.New("invalid youtube Url"))
	}
	cResp, err = channelListCall.Do()

	if err != nil {
		return nil, common.ErrWithCaller(err)
	}
	if len(cResp.Items) < 1 {
		return nil, ErrNoChannel
	}
	return cResp.Items[0], nil
}

func (p *Plugin) AddFeed(guildID, discordChannelID int64, ytChannel *youtube.Channel, mentionEveryone bool, publishLivestream bool, publishShorts bool, mentionRoles []int64) (*ChannelSubscription, error) {
	if mentionEveryone && len(mentionRoles) > 0 {
		mentionRoles = make([]int64, 0)
	}

	sub := &ChannelSubscription{
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
	logger.Info("Added websub")
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

	resp, err := p.YTService.Videos.List([]string{"snippet", "contentDetails"}).Id(videoID).Do()
	if err != nil || len(resp.Items) < 1 {
		return err
	}

	item := resp.Items[0]
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

func (p *Plugin) isShortsVideo(video *youtube.Video) bool {
	if video.Snippet.LiveBroadcastContent == "live" {
		return false
	}
	videoDurationString := strings.ToLower(strings.TrimPrefix(video.ContentDetails.Duration, "PT"))
	videoDuration, err := common.ParseDuration(videoDurationString)
	if err != nil {
		logger.WithError(err).Errorf("Failed to parse video duration with value %s, assuming it is not a short video", videoDurationString)
		return false
	}
	if videoDuration > time.Minute {
		return false
	}

	return p.isShortsRedirect(video.Id)
}

func (p *Plugin) isShortsRedirect(videoId string) bool {
	shortsUrl := fmt.Sprintf("https://www.youtube.com/shorts/%s", videoId)
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

func (p *Plugin) postVideo(subs []*ChannelSubscription, publishedAt time.Time, video *youtube.Video, channelID string) error {
	err := common.MultipleCmds(
		radix.FlatCmd(nil, "SET", KeyLastVidTime(channelID), publishedAt.Unix()),
		radix.FlatCmd(nil, "SET", KeyLastVidID(channelID), video.Id),
	)
	if err != nil {
		return err
	}
	isShortsCheckDone := false
	isShorts := false
	for _, sub := range subs {
		if sub.Enabled {
			if !sub.PublishShorts {
				//extra check to only check if a video is a short when seeing the first shorts disabled subscription
				//this is just a flag to check if the check was done and the "isShorts" variable actually has a verified value
				//since "PublishShorts" is enabled by default
				if !isShortsCheckDone {
					isShorts = p.isShortsVideo(video)
					isShortsCheckDone = true
				}

				if isShorts {
					continue
				}
			}

			if video.Snippet.LiveBroadcastContent == "live" && !sub.PublishLivestream {
				continue
			}
			p.sendNewVidMessage(sub, video)
		}
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
