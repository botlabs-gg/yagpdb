package reddit

//go:generate sqlboiler psql

import (
	"context"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/go-reddit"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/reddit/models"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
)

const (
	FilterNSFWNone    = 0 // allow both nsfw and non nsfw content
	FilterNSFWIgnore  = 1 // only allow non-nsfw content
	FilterNSFWRequire = 2 // only allow nsfw content
)

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

// Remove feeds if they don't point to a proper channel
func (p *Plugin) HandleMQueueError(elem *mqueue.QueuedElement, err error) {
	code, _ := common.DiscordError(err)
	if code != discordgo.ErrCodeUnknownChannel && code != discordgo.ErrCodeMissingAccess && code != discordgo.ErrCodeMissingPermissions {
		l := log.WithError(err).WithField("channel", elem.Channel)
		l = l.WithField("s_msg", elem.MessageEmbed)

		l.Warn("[reddit] Error posting reddit message")
		return
	}

	if strings.Contains(elem.SourceID, ":") {
		// legacy format leftover, ignore...
		return
	}

	log.WithError(err).WithField("channel", elem.Channel).WithField("id", elem.SourceID).Info("[reddit] Disabling reddit feed to nonexistant discord channel")

	feedID, err := strconv.ParseInt(elem.SourceID, 10, 64)
	if err != nil {
		log.WithError(err).WithField("source_id", elem.SourceID).Error("[reddit] failed parsing sourceID!??!")
		return
	}

	_, err = models.RedditFeeds(models.RedditFeedWhere.ID.EQ(feedID)).UpdateAllG(context.Background(), models.M{"disabled": true})
	if err != nil {
		log.WithError(err).WithField("feed_id", feedID).Error("[reddit] failed removing reddit feed")
	}
}

var _ mqueue.PluginWithWebhookAvatar = (*Plugin)(nil)

func (p *Plugin) WebhookAvatar() string {
	return RedditLogoPNGB64
}

func RegisterPlugin() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		log.WithError(err).Error("reddit: failed initalizing database schema, not enabling plugin")
		return
	}

	plugin := &Plugin{
		stopFeedChan: make(chan *sync.WaitGroup),
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
