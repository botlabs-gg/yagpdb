package twitter

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/twitter/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/volatiletech/sqlboiler/boil"
)

var _ feeds.Plugin = (*Plugin)(nil)

func (p *Plugin) StartFeed() {
	p.Stop = make(chan *sync.WaitGroup)
	go p.updateConfigsLoop()
	p.runFeedLoop()
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

func (p *Plugin) runFeedLoop() {

	var currentFeeds []*models.TwitterFeed
	var stream *twitter.Stream

	stoppedCheck := new(int32)

	ticker := time.NewTicker(time.Minute)
	startDelay := time.After(time.Second * 2)
	var lastStart time.Time
	for {
		select {
		case <-startDelay:
		case wg := <-p.Stop:
			if stream != nil {
				stream.Stop()
			}
			wg.Done()
			return
		case <-ticker.C:
		}

		// check if we need to restart it cause of new or removed feeds
		p.feedsLock.Lock()
		newFeeds := p.feeds
		p.feedsLock.Unlock()

		if !feedsChanged(currentFeeds, newFeeds) && stream != nil && atomic.LoadInt32(stoppedCheck) == 0 || time.Since(lastStart) < time.Minute*10 {
			continue
		}

		logger.Info("Feeds changed or stopped, restarting...")

		// restart
		if stream != nil {
			stream.Stop()
		}
		stream = nil

		if len(newFeeds) == 0 {
			continue
		}

		stoppedCheck = new(int32)
		currentFeeds = newFeeds
		var err error
		stream, err = p.runFeed(newFeeds, stoppedCheck)
		if err != nil {
			logger.WithError(err).Error("failed starting stream")
			time.Sleep(time.Second * 10)
		} else {
			lastStart = time.Now()
		}
	}
}

func (p *Plugin) runFeed(feeds []*models.TwitterFeed, stoppedCheck *int32) (*twitter.Stream, error) {

	follow := make([]string, 0, len(feeds))

OUTER:
	for _, v := range feeds {
		strID := strconv.FormatInt(v.TwitterUserID, 10)

		for _, existing := range follow {

			if strID == existing {
				continue OUTER
			}
		}

		follow = append(follow, strID)
	}

	params := &twitter.StreamFilterParams{
		StallWarnings: twitter.Bool(true),
		Follow:        follow,
		// Track: []string{"cute"},
	}

	stream, err := p.twitterAPI.Streams.Filter(params)
	if err != nil {
		return nil, err
	}

	go p.handleStream(stream, stoppedCheck)
	return stream, nil
}

func (p *Plugin) handleStream(stream *twitter.Stream, stoppedCheck *int32) {
	defer atomic.StoreInt32(stoppedCheck, 1)

	logger.Info("listening for events")
	for m := range stream.Messages {

		switch t := m.(type) {
		case *twitter.Tweet:
			go p.handleTweet(t)
		default:
			logger.Info("Unknown event: ", m)
		}
	}
	logger.Info("stopped listening for events")
}

func (p *Plugin) handleTweet(t *twitter.Tweet) {
	if t.User == nil {
		logger.Errorf("Twitter user is nil?: %#v", t)
		return
	}

	p.feedsLock.Lock()
	tFeeds := p.feeds
	p.feedsLock.Unlock()

	relevantFeeds := make([]*models.TwitterFeed, 0, 4)

OUTER:
	for _, f := range tFeeds {
		if t.User.ID != f.TwitterUserID {
			continue
		}

		for _, r := range relevantFeeds {
			// skip multiple feeds to the same channel
			if f.ChannelID == r.ChannelID {
				continue OUTER
			}
		}

		isRetweet := t.RetweetedStatus != nil
		if isRetweet && !f.IncludeRT {
			continue
		}

		isReply := t.InReplyToScreenName != "" || t.InReplyToStatusID != 0 || t.InReplyToUserID != 0
		if isReply && !f.IncludeReplies {
			continue
		}

		relevantFeeds = append(relevantFeeds, f)
	}

	if len(relevantFeeds) < 1 {
		return
	}

	webhookUsername := t.User.ScreenName + " • YAGPDB"
	embed := createTweetEmbed(t)
	for _, v := range relevantFeeds {
		go analytics.RecordActiveUnit(v.GuildID, p, "posted_twitter_message")

		mqueue.QueueMessage(&mqueue.QueuedElement{
			Source:   "twitter",
			SourceID: strconv.FormatInt(v.ID, 10),

			Guild:   v.GuildID,
			Channel: v.ChannelID,

			MessageEmbed:    embed,
			UseWebhook:      true,
			WebhookUsername: webhookUsername,

			Priority: 5, // above youtube and reddit
		})
	}

	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "twitter"}).Add(float64(len(relevantFeeds)))

	logger.Infof("Handled tweet %q on %d channels", t.Text, len(relevantFeeds))
}

func createTweetEmbed(tweet *twitter.Tweet) *discordgo.MessageEmbed {
	timeStr := ""
	if parsed, err := tweet.CreatedAtTime(); err == nil {
		timeStr = parsed.Format(time.RFC3339)
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "@" + tweet.User.ScreenName,
			IconURL: tweet.User.ProfileImageURLHttps,
			URL:     "https://twitter.com/" + tweet.User.ScreenName + "/status/" + tweet.IDStr,
		},
		Description: tweet.Text,
		Timestamp:   timeStr,
		Color:       0x38A1F3,
	}

	if tweet.Entities != nil && len(tweet.Entities.Media) > 0 {
		m := tweet.Entities.Media[0]
		if m.Type == "photo" || m.Type == "animated_gif" {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: m.MediaURLHttps,
			}
		}
	}

	serialised, _ := json.Marshal(embed)
	logger.Info(string(serialised))

	return embed
}

func feedsChanged(a, b []*models.TwitterFeed) bool {
	if len(a) != len(b) {
		return true
	}

	// check if theres some in a but not in b
OUTER:
	for _, va := range a {
		for _, vb := range b {
			if va.ID == vb.ID {
				continue OUTER
			}
		}

		// not found
		return true
	}

	// check the opposite
OUTER2:
	for _, vb := range b {
		for _, va := range a {
			if va.ID == vb.ID {
				continue OUTER2
			}
		}

		// not found
		return true
	}

	return false
}

func (p *Plugin) updateConfigsLoop() {
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	for {
		p.updateConfigs()

		select {
		case <-ticker.C:
		case wg := <-p.Stop:
			wg.Done()
			logger.Info("youtube updateConfigsLoop shut down")
			return
		}
	}
}

func (p *Plugin) updateConfigs() {
	configs, err := models.TwitterFeeds(models.TwitterFeedWhere.Enabled.EQ(true)).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("failed updating configs")
		return
	}

	filtered := make([]*models.TwitterFeed, 0, len(configs))
	for _, v := range configs {
		isPremium, err := premium.IsGuildPremium(v.GuildID)
		if err != nil {
			logger.WithError(err).Error("failed checking if guild is premium")
			return
		}

		if !isPremium {
			v.Enabled = false
			_, err = v.UpdateG(context.Background(), boil.Whitelist("enabled"))
			if err != nil {
				logger.WithError(err).Error("failed disabling non-premium feed")
			}
			continue
		}

		filtered = append(filtered, v)
	}

	p.feedsLock.Lock()
	p.feeds = filtered
	p.feedsLock.Unlock()
}
