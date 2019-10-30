package reddit

//go:generate sqlboiler psql

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/jonas747/go-reddit"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/reddit/models"
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
	if strings.Contains(elem.SourceID, ":") {
		// legacy format leftover, ignore...
		return
	}

	feedID, err := strconv.ParseInt(elem.SourceID, 10, 64)
	if err != nil {
		logger.WithError(err).WithField("source_id", elem.SourceID).Error("failed parsing sourceID!??!")
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
