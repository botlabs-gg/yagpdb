package reddit

//go:generate sqlboiler psql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/common/mqueue"
	"github.com/botlabs-gg/yagpdb/common/pubsub"
	"github.com/botlabs-gg/yagpdb/premium"
	"github.com/botlabs-gg/yagpdb/reddit/models"
	"github.com/jonas747/go-reddit"
)

const (
	FilterNSFWNone    = 0 // allow both nsfw and non nsfw content
	FilterNSFWIgnore  = 1 // only allow non-nsfw content
	FilterNSFWRequire = 2 // only allow nsfw content
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct {
	stopFeedChan chan *sync.WaitGroup
	redditClient *reddit.Client
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Reddit",
		SysName:  "reddit",
		Category: common.PluginCategoryFeeds,
	}
}

var _ mqueue.PluginWithSourceDisabler = (*Plugin)(nil)

// Remove feeds if they don't point to a proper channel
func (p *Plugin) DisableFeed(elem *mqueue.QueuedElement, err error) {
	if strings.Contains(elem.SourceItemID, ":") {
		// legacy format leftover, ignore...
		return
	}

	feedID, err := strconv.ParseInt(elem.SourceItemID, 10, 64)
	if err != nil {
		logger.WithError(err).WithField("source_id", elem.SourceItemID).Error("failed parsing sourceID!??!")
		return
	}

	_, err = models.RedditFeeds(models.RedditFeedWhere.ID.EQ(feedID)).UpdateAllG(context.Background(), models.M{"disabled": true})
	if err != nil {
		logger.WithError(err).WithField("feed_id", feedID).Error("failed removing reddit feed")
	}
}

var _ mqueue.PluginWithWebhookAvatar = (*Plugin)(nil)

func (p *Plugin) WebhookAvatar() string {
	return RedditLogoPNGB64
}

func RegisterPlugin() {
	common.InitSchemas("reddit", DBSchemas...)

	plugin := &Plugin{
		stopFeedChan: make(chan *sync.WaitGroup),
	}

	if confClientID.GetString() == "" || confClientSecret.GetString() == "" || confRefreshToken.GetString() == "" {
		logger.Warn("Missing reddit config, not enabling plugin")
		return
	}

	plugin.redditClient = setupClient()

	common.RegisterPlugin(plugin)
	mqueue.RegisterSource("reddit", plugin)

	pubsub.AddHandler("reddit_clear_subreddit_cache", func(evt *pubsub.Event) {
		dataCast := evt.Data.(*PubSubSubredditEventData)
		if dataCast.Slow {
			configCache.Delete(KeySlowFeeds(strings.ToLower(dataCast.Subreddit)))
		} else {
			configCache.Delete(KeyFastFeeds(strings.ToLower(dataCast.Subreddit)))
		}
	}, PubSubSubredditEventData{})
}

type PubSubSubredditEventData struct {
	Subreddit string `json:"subreddit"`
	Slow      bool   `json:"slow"`
}

const (
	// Max feeds per guild
	GuildMaxFeedsNormal  = 100
	GuildMaxFeedsPremium = 1000
)

func MaxFeedForCtx(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return GuildMaxFeedsPremium
	}

	return GuildMaxFeedsNormal
}

func CheckSubreddit(name string) bool {
	var redditData struct {
		Data struct {
			Dist     int `json:"dist"`
			Children []struct {
			}
		} `json:"data"`
	}

	query := "https://api.reddit.com/subreddits/search.api?q=" + name
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", UserAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	err = json.Unmarshal(body, &redditData)
	if err != nil {
		return false
	}

	return redditData.Data.Dist > 0 && len(redditData.Data.Children) > 0
}
