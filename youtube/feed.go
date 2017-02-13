package youtube

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
	"sync"
	"time"
)

const (
	PollInterval = time.Second * 100
	// PollInterval       = time.Second * 5 // <- used for debug purposes
	MaxChannelsPerPoll = 100 // Can probably be safely increased to 1000 when caches are hot
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
			err := p.checkChannels(redisClient)
			if err != nil {
				logrus.WithError(err).Error("Failed checking youtube channels")
			}
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
			logrus.WithError(err).WithField("yt_channel", channel).Error("Failed checking youtube channel")
		}
	}

	return nil
}

func (p *Plugin) checkChannel(client *redis.Client, channel string) error {
	now := time.Now()

	var subs []*ChannelSubscription
	err := common.SQL.Where("youtube_channel_id = ?", channel).Find(&subs).Error
	if err != nil {
		return err
	}

	if len(subs) < 1 {
		logrus.Info("No subs?")
		time.AfterFunc(time.Second, func() {
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

	var latestVid *youtube.PlaylistItem

	first := true

	for {
		call := p.YTService.PlaylistItems.List("snippet").PlaylistId(playlistID)
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

		done, err := p.handlePlaylistItemsResponse(resp, subs, lastProcessedVidTime, lastVidID)
		if err != nil {
			return err
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

	if latestVid != nil && lastVidID != latestVid.Id {
		parsedTime, _ := time.Parse(time.RFC3339, latestVid.Snippet.PublishedAt)
		client.Cmd("SET", KeyLastVidTime(channel), parsedTime.Unix())
		client.Cmd("SET", KeyLastVidID(channel), latestVid.Id)
	}

	return nil
}

func (p *Plugin) handlePlaylistItemsResponse(resp *youtube.PlaylistItemListResponse, subs []*ChannelSubscription, lastProcessedVidTime time.Time, lastVidID string) (complete bool, err error) {
	for _, item := range resp.Items {
		parsedPublishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if err != nil {
			logrus.WithError(err).Error("Failed parsing video time")
			continue
		}

		if lastProcessedVidTime.After(parsedPublishedAt) || item.Id == lastVidID {
			complete = true
			break
		}

		for _, sub := range subs {
			go p.sendNewVidMessage(sub.ChannelID, item)
		}
	}

	return
}

func (p *Plugin) sendNewVidMessage(discordChannel string, item *youtube.PlaylistItem) {
	content := fmt.Sprintf("**%s** Uploaded a new youtube video!\n%s", item.Snippet.ChannelTitle, "https://www.youtube.com/watch?v="+item.Snippet.ResourceId.VideoId)
	common.BotSession.ChannelMessageSend(discordChannel, content)
}

var (
	ErrIDNotFound = errors.New("ID not found")
)

func (p *Plugin) PlaylistID(channelID string) (string, error) {

	var entry YoutubePlaylistID
	err := common.SQL.Where("channel_id = ?", channelID).First(&entry).Error
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

	common.SQL.Create(&entry)

	return id, nil
}

func SubsForChannel(channel string) (result []*ChannelSubscription, err error) {
	err = common.SQL.Where("youtube_channel_id = ?", channel).Find(&result).Error
	return
}

var (
	ErrNoChannel = errors.New("No channel with that id found")
)

func (p *Plugin) AddFeed(client *redis.Client, guildID, discordChannelID, youtubeChannelID, youtubeUsername string) (*ChannelSubscription, error) {
	sub := &ChannelSubscription{
		GuildID:   guildID,
		ChannelID: discordChannelID,
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

	err = common.BlockingLockRedisKey(client, RedisChannelsLockKey, 5)
	if err != nil {
		return nil, err
	}
	defer common.UnlockRedisKey(client, RedisChannelsLockKey)

	err = common.SQL.Create(sub).Error
	if err != nil {
		return nil, err
	}

	err = maybeAddChannelWatch(false, client, youtubeChannelID)
	return sub, err
}

// maybeRemoveChannelWatch checks the channel for subs, if it has none then it removes it from the watchlist in redis.
func maybeRemoveChannelWatch(channel string) {
	client, err := common.RedisPool.Get()
	if err != nil {
		return
	}
	defer common.RedisPool.Put(client)

	err = common.BlockingLockRedisKey(client, RedisChannelsLockKey, 5)
	if err != nil {
		return
	}
	defer common.UnlockRedisKey(client, RedisChannelsLockKey)

	var count int
	err = common.SQL.Model(&ChannelSubscription{}).Where("youtube_channel_id = ?", channel).Count(&count).Error
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
		err := common.BlockingLockRedisKey(client, RedisChannelsLockKey, 5)
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
		return nil
	}

	client.Cmd("ZADD", "youtube_subbed_channels", now, channel)
	client.Cmd("SET", KeyLastVidTime(channel), now)
	logrus.WithField("yt_channel", channel).Info("Added new youtube channel watch")
	return nil
}
