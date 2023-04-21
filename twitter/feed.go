package twitter

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/twitter/models"
	"github.com/mediocregopher/radix/v3"
	twitterscraper "github.com/n0madic/twitter-scraper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var _ feeds.Plugin = (*Plugin)(nil)

func KeyLastTweetTime(id string) string { return "twitter_last_tweet_time:" + id }
func KeyLastTweetID(id string) string   { return "twitter_last_tweet_id:" + id }

func (p *Plugin) StartFeed() {
	logrus.Info("STARTING TWITTER FEED")
	p.Stop = make(chan *sync.WaitGroup)
	go p.updateConfigsLoop()
	go p.runFeedLoop()
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
	logrus.Info("STARTING TWITTER FEED LOOP")
	ticker := time.NewTicker(time.Minute)
	startDelay := time.After(time.Second * 2)
	for {
		select {
		case <-ticker.C:
			logrus.Info("TWITTER LOOP TICKER")
			p.feedsLock.Lock()
			newFeeds := p.feeds
			p.feedsLock.Unlock()
			p.runFeed(newFeeds)
		case <-startDelay:
			logrus.Info("TWITTER LOOP DELAY")
		case wg := <-p.Stop:
			logrus.Info("TWITTER LOOP STOP")
			wg.Done()
			return
		}
		logrus.Info("TWITTER LOOP RUNNING")
		// check if we need to restart it cause of new or removed feeds
	}
}

func (p *Plugin) getLastTweetInfo(username string) (tweetId string, tweetTime time.Time, err error) {
	// Find the last video time for this channel
	var unixSeconds int64
	err = common.RedisPool.Do(radix.Cmd(&unixSeconds, "GET", KeyLastTweetTime(username)))

	var lastProcessedTweetTime time.Time
	if err != nil || unixSeconds == 0 {
		lastProcessedTweetTime = time.Time{}
	} else {
		lastProcessedTweetTime = time.Unix(unixSeconds, 0)
	}

	var lastTweetID string
	err = common.RedisPool.Do(radix.Cmd(&lastTweetID, "GET", KeyLastTweetID(username)))
	return lastTweetID, lastProcessedTweetTime, err
}

func (p *Plugin) checkTweet(tweet *twitterscraper.Tweet) {
	lastTweetID, lastTweetTime, err := p.getLastTweetInfo(tweet.Username)
	if err != nil {
		logrus.WithError(err).Errorf("Failed getting last tweet info for username %s", tweet.Username)
		return
	}

	if lastTweetID == tweet.ID {
		// the tweet has already been processed
		return
	}

	if time.Since(tweet.TimeParsed) > time.Hour {
		// just a safeguard against empty last tweet time's
		return
	}

	if lastTweetTime.After(tweet.TimeParsed) {
		// wasn't a new tweet
		return
	}

	// This is a new tweet, post it
	p.handleTweet(tweet)
}

func (p *Plugin) getTweetsForUser(username string) {
	logrus.Infof("Getting TWEET for user %s", username)
	for tweet := range p.twitterScraper.GetTweets(context.Background(), username, 50) {
		if tweet.Error != nil {
			logrus.WithError(tweet.Error).Errorf("Failed Getting Tweet for user %s", username)
			continue
		}
		go p.checkTweet(&tweet.Tweet)
	}
}

func (p *Plugin) runFeed(feeds []*models.TwitterFeed) {
	uniqueFeeds := make(map[string]int)
	logrus.Info("RUNNING TWITTER FEED")
	for _, v := range feeds {
		if uniqueFeeds[v.TwitterUsername] == 0 {
			uniqueFeeds[v.TwitterUsername] = 1
		}
		uniqueFeeds[v.TwitterUsername]++
	}

	logger.Info("NUMBER OF Unique Feeds: ", len(uniqueFeeds))
	for user := range uniqueFeeds {
		go p.getTweetsForUser(user)
	}
}

func (p *Plugin) handleTweet(t *twitterscraper.Tweet) {
	logrus.Infof("Handling Tweet %s", t.Text)
	if t.UserID == "" {
		logger.Errorf("Twitter user is nil?: %#v", t)
		return
	}

	p.feedsLock.Lock()
	tFeeds := p.feeds
	p.feedsLock.Unlock()

	relevantFeeds := make([]*models.TwitterFeed, 0, 4)

OUTER:
	for _, f := range tFeeds {
		tweetUser, _ := strconv.ParseInt(t.UserID, 10, 64)
		if tweetUser != f.TwitterUserID {
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

		if t.IsReply && !f.IncludeReplies {
			continue
		}

		relevantFeeds = append(relevantFeeds, f)
	}

	if len(relevantFeeds) < 1 {
		return
	}
	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyLastTweetTime(t.Username), time.Now().Unix()))
	if err != nil {
		logrus.WithError(err).Errorf("Failed Saving tweet time for %s ", t.Username)
		return
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyLastTweetID(t.ID), time.Now().Unix()))
	if err != nil {
		logrus.WithError(err).Errorf("Failed Saving tweet id for %s ", t.Username)
		return
	}

	webhookUsername := "Twitter • YAGPDB"
	embed := createTweetEmbed(t)
	for _, v := range relevantFeeds {
		go analytics.RecordActiveUnit(v.GuildID, p, "posted_twitter_message")

		mqueue.QueueMessage(&mqueue.QueuedElement{
			Source:       "twitter",
			SourceItemID: strconv.FormatInt(v.ID, 10),

			GuildID:   v.GuildID,
			ChannelID: v.ChannelID,

			MessageEmbed:    embed,
			UseWebhook:      true,
			WebhookUsername: webhookUsername,

			Priority: 5, // above youtube and reddit
		})
	}

	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "twitter"}).Add(float64(len(relevantFeeds)))

	logger.Infof("Handled tweet %q on %d channels", t.Text, len(relevantFeeds))
}

func createTweetEmbed(tweet *twitterscraper.Tweet) *discordgo.MessageEmbed {
	timeStr := time.Unix(tweet.Timestamp, 0).Format(time.RFC3339)
	text := tweet.Text
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: "@" + tweet.Username,
			URL:  tweet.PermanentURL,
		},
		Description: text,
		Timestamp:   timeStr,
		Color:       0x38A1F3,
	}

	if tweet.Photos != nil && len(tweet.Photos) > 0 {
		m := tweet.Photos[0]
		embed.Image = &discordgo.MessageEmbedImage{
			URL: m,
		}
	}

	return embed
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
			logger.Info("Twitter updateConfigsLoop shut down")
			return
		}
	}
}

func (p *Plugin) updateConfigs() {
	configs, err := models.TwitterFeeds(models.TwitterFeedWhere.Enabled.EQ(true), qm.OrderBy("id asc")).AllG(context.Background())
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
