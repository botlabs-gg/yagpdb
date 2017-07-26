package youtube

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
	"sync"
	"time"
)

const (
	MaxChannelsPerPoll = 200 // Can probably be safely increased to 1000 when caches are hot
	PollInterval       = time.Second * 100
	// PollInterval = time.Second * 5 // <- used for debug purposes
)

func (p *Plugin) StartFeed() {
	p.Stop = make(chan *sync.WaitGroup)
	p.runFeed()
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

func (p *Plugin) runFeed() {
	redisClient := common.MustGetRedisClient()

	ticker := time.NewTicker(PollInterval)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			now := time.Now()
			err := p.checkChannels(redisClient)
			if err != nil {
				logrus.WithError(err).Error("Failed checking youtube channels")
			}
			logrus.Info("Took", time.Since(now), "to check youtube feeds")
		}
	}
}

func (p *Plugin) checkChannels(client *redis.Client) error {
	channels, err := client.Cmd("ZRANGE", "youtube_subbed_channels", 0, MaxChannelsPerPoll).List()
	if err != nil {
		return err
	}

	for _, channel := range channels {
		err = p.checkChannel(client, channel)
		if err != nil {
			if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
				logrus.WithError(err).WithField("yt_channel", channel).Warn("Removing non existant youtube channel")
				err = common.GORM.Where("youtube_channel_id = ?", channel).Delete(ChannelSubscription{}).Error
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.WithError(err).Error("Failed deleting nonexistant channel subs")
				}
				go maybeRemoveChannelWatch(channel)
			} else {
				logrus.WithError(err).WithField("yt_channel", channel).Error("Failed checking youtube channel")
			}
		}
	}

	return nil
}

func (p *Plugin) checkChannel(client *redis.Client, channel string) error {
	now := time.Now()

	var subs []*ChannelSubscription
	err := common.GORM.Where("youtube_channel_id = ?", channel).Find(&subs).Error
	if err != nil {
		return err
	}

	if len(subs) < 1 {
		time.AfterFunc(time.Second*10, func() {
			maybeRemoveChannelWatch(channel)
		})
		return nil
	}

	playlistID, err := p.PlaylistID(channel)
	if err != nil {
		return err
	}

	nextPage := ""
	seconds, err := client.Cmd("GET", KeyLastVidTime(channel)).Int64()
	var lastProcessedVidTime time.Time
	if err != nil {
		logrus.WithError(err).Error("Failed retrieving last processed vid time, falling back to this time")
		lastProcessedVidTime = time.Now()
	} else {
		lastProcessedVidTime = time.Unix(seconds, 0)
	}

	lastVidID, _ := client.Cmd("GET", KeyLastVidID(channel)).Str()

	// latestVid is used to set the last vid id and time
	var latestVid *youtube.PlaylistItem

	first := true

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
				logrus.WithError(err).WithField("vid", latestVid.Id).Error("Failed parsing publishedat")
			} else {
				if parsedPublishedAtLv.After(parsedPublishedOld) {
					latestVid = lv
				}
			}
		}

		if done {
			break
		}

		logrus.Debug("next", resp.NextPageToken)
		if resp.NextPageToken == "" {
			break // Reached end
		}
		nextPage = resp.NextPageToken
	}

	client.Cmd("ZADD", "youtube_subbed_channels", now.Unix(), channel)

	// Update the last vid id and time if needed
	if latestVid != nil && lastVidID != latestVid.Id {
		parsedTime, _ := time.Parse(time.RFC3339, latestVid.Snippet.PublishedAt)
		if !lastProcessedVidTime.After(parsedTime) {
			client.Cmd("SET", KeyLastVidTime(channel), parsedTime.Unix())
			client.Cmd("SET", KeyLastVidID(channel), latestVid.Id)
		}
	}

	return nil
}

func (p *Plugin) handlePlaylistItemsResponse(resp *youtube.PlaylistItemListResponse, subs []*ChannelSubscription, lastProcessedVidTime time.Time, lastVidID string) (latest *youtube.PlaylistItem, complete bool, err error) {

	var latestTime time.Time

	for _, item := range resp.Items {
		parsedPublishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if err != nil {
			logrus.WithError(err).Error("Failed parsing video time")
			continue
		}

		// Video is published before the latest video we checked, mark as complete and do not post messages for
		if !parsedPublishedAt.After(lastProcessedVidTime) || item.Id == lastVidID {
			complete = true
			continue
		}

		// This is the new latest video
		if parsedPublishedAt.After(latestTime) {
			latestTime = parsedPublishedAt
			latest = item
		}

		for _, sub := range subs {
			go p.sendNewVidMessage(sub.ChannelID, item, sub.MentionEveryone)
		}
	}

	return
}

func (p *Plugin) sendNewVidMessage(discordChannel string, item *youtube.PlaylistItem, mentionEveryone bool) {
	content := common.EscapeEveryoneMention(fmt.Sprintf("**%s** Uploaded a new youtube video!\n%s", item.Snippet.ChannelTitle, "https://www.youtube.com/watch?v="+item.Snippet.ResourceId.VideoId))
	if mentionEveryone {
		content += " @everyone"
	}
	err := common.RetrySendMessage(discordChannel, content, 50)
	if err != nil {
		if rError, ok := err.(*discordgo.RESTError); ok && rError.Response.StatusCode == 404 {
			// Tried to send to nonexistant channel, remove all subs to this channel.
			err = common.GORM.Where("channel_id = ?", discordChannel).Delete(ChannelSubscription{}).Error
			if err != nil {
				logrus.WithError(err).Error("failed removing nonexistant channel")
			}
		} else {
			logrus.WithError(err).WithField("channel", discordChannel).Error("Failed sending youtube sub message")
		}
	}
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

func (p *Plugin) AddFeed(client *redis.Client, guildID, discordChannelID, youtubeChannelID, youtubeUsername string, mentionEveryone bool) (*ChannelSubscription, error) {
	sub := &ChannelSubscription{
		GuildID:         guildID,
		ChannelID:       discordChannelID,
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

	err = common.BlockingLockRedisKey(client, RedisChannelsLockKey, 0, 5)
	if err != nil {
		return nil, err
	}
	defer common.UnlockRedisKey(client, RedisChannelsLockKey)

	err = common.GORM.Create(sub).Error
	if err != nil {
		return nil, err
	}

	err = maybeAddChannelWatch(false, client, sub.YoutubeChannelID)
	return sub, err
}

// maybeRemoveChannelWatch checks the channel for subs, if it has none then it removes it from the watchlist in redis.
func maybeRemoveChannelWatch(channel string) {
	client, err := common.RedisPool.Get()
	if err != nil {
		return
	}
	defer common.RedisPool.Put(client)

	err = common.BlockingLockRedisKey(client, RedisChannelsLockKey, 0, 5)
	if err != nil {
		return
	}
	defer common.UnlockRedisKey(client, RedisChannelsLockKey)

	var count int
	err = common.GORM.Model(&ChannelSubscription{}).Where("youtube_channel_id = ?", channel).Count(&count).Error
	if err != nil || count > 0 {
		if err != nil {
			logrus.WithError(err).WithField("yt_channel", channel).Error("Failed getting sub count")
		}
		return
	}

	err = client.Cmd("ZREM", "youtube_subbed_channels", channel).Err
	client.Cmd("DEL", KeyLastVidTime(channel))
	client.Cmd("DEL", KeyLastVidID(channel))
	if err != nil {
		return
	}

	logrus.WithField("yt_channel", channel).Info("Removed orphaned youtube channel from subbed channel sorted set")
}

// maybeAddChannelWatch adds a channel watch to redis, if there wasnt one before
func maybeAddChannelWatch(lock bool, client *redis.Client, channel string) error {
	if lock {
		err := common.BlockingLockRedisKey(client, RedisChannelsLockKey, 0, 5)
		if err != nil {
			return common.ErrWithCaller(err)
		}
		defer common.UnlockRedisKey(client, RedisChannelsLockKey)
	}

	now := time.Now().Unix()

	reply := client.Cmd("ZSCORE", "youtube_subbed_channels", channel)
	if reply.Err != nil {
		return common.ErrWithCaller(reply.Err)
	}

	if reply.Type != redis.NilReply {
		// already added before, don't need to do anything
		logrus.Info("Not nil reply", reply.String())
		return nil
	}

	client.Cmd("ZADD", "youtube_subbed_channels", now, channel)
	client.Cmd("SET", KeyLastVidTime(channel), now)
	logrus.WithField("yt_channel", channel).Info("Added new youtube channel watch")
	return nil
}
