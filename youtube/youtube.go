package youtube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/youtube/models"
	"github.com/mediocregopher/radix/v3"
	"google.golang.org/api/youtube/v3"
)

//go:generate sqlboiler --no-hooks psql

const (
	RedisChannelsLockKey       = "youtube_subbed_channel_lock"
	RedisKeyPublishedVideoList = "youtube_published_videos"
	RedisKeyWebSubChannels     = "youtube_registered_websub_channels"
	GoogleWebsubHub            = "https://pubsubhubbub.appspot.com/subscribe"
)

var (
	confWebsubVerifytoken     = config.RegisterOption("yagpdb.youtube.verify_token", "Youtube websub push verify token, set it to a random string and never change it", "asdkpoasdkpaoksdpako")
	confResubBatchSize        = config.RegisterOption("yagpdb.youtube.resub_batch_size", "Number of Websubs to resubscribe to concurrently", 1)
	confWebsubRequestsMinute  = config.RegisterOption("yagpdb.youtube.websub_requests_minute", "Max subscribe requests per minute sent to the pubsubhubbub hub, which rate limits by source ip", 30)
	confYoutubeVideoCacheDays = config.RegisterOption("yagpdb.youtube.video_cache_duration", "Duration in days to cache youtube video data", 1)
	logger                    = common.GetPluginLogger(&Plugin{})
)

func KeyLastVidTime(channel string) string { return "youtube_last_video_time:" + channel }
func KeyLastVidID(channel string) string   { return "youtube_last_video_id:" + channel }

type Plugin struct {
	YTService *youtube.Service
	Stop      chan *sync.WaitGroup
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Youtube",
		SysName:  "youtube",
		Category: common.PluginCategoryFeeds,
	}
}

func RegisterPlugin() {
	p := &Plugin{}

	mqueue.RegisterSource("youtube", p)

	err := p.SetupClient()
	if err != nil {
		logger.WithError(err).Error("Failed setting up youtube plugin, youtube plugin will not be enabled.")
		return
	}
	common.RegisterPlugin(p)

	common.InitSchemas("youtube", DBSchemas...)
}

var _ mqueue.PluginWithSourceDisabler = (*Plugin)(nil)

// Remove feeds if they don't point to a proper channel
func (p *Plugin) DisableFeed(elem *mqueue.QueuedElement, err error) {
	p.DisableChannelFeeds(elem.ChannelID)
}

func (p *Plugin) DisableChannelFeeds(channelID int64) error {
	numDisabled, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.ChannelID.EQ(discordgo.StrID(channelID)),
	).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("channel", channelID).Error("failed removing feeds in nonexistent channel")
		return err
	}

	logger.WithField("channel", channelID).Infof("disabled %d feeds in nonexistent channel", numDisabled)
	return nil
}

func (p *Plugin) DisableGuildFeeds(guildID int64) error {
	numDisabled, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(guildID)),
	).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed removing feeds in nonexistent guild")
		return err
	}

	logger.WithField("guild", guildID).Infof("disabled %d feeds in nonexistent guild", numDisabled)
	return nil
}

var websubClient = &http.Client{Timeout: time.Second * 30}

const (
	// only the hub's verification callback advances the real score, so without this
	// every unverified channel is reposted on the next tick
	websubVerifyGracePeriod = time.Minute * 5

	// the hub keeps answering 503 for a while once tripped, at ~20s per request
	websubRateLimitCooldown = time.Minute * 10

	websubMaxQueueWait = time.Second * 15
)

var errHubCoolingDown = errors.New("hub rate limited us, waiting out the cooldown")

// hubGate paces subscribe requests to the hub, which rate limits by source ip.
type hubGate struct {
	mu          sync.Mutex
	next        time.Time
	cooldownEnd time.Time
}

var websubGate = &hubGate{}

// reserve blocks until the caller's turn, reporting false if the hub is cooling down
// or the queue is already longer than websubMaxQueueWait.
func (g *hubGate) reserve() bool {
	g.mu.Lock()

	now := time.Now()
	if now.Before(g.cooldownEnd) {
		g.mu.Unlock()
		return false
	}

	slot := g.next
	if slot.Before(now) {
		slot = now
	}

	if slot.Sub(now) > websubMaxQueueWait {
		g.mu.Unlock()
		return false
	}

	interval := time.Minute / time.Duration(max(confWebsubRequestsMinute.GetInt(), 1))
	g.next = slot.Add(interval)
	g.mu.Unlock()

	time.Sleep(time.Until(slot))

	return true
}

func (g *hubGate) rateLimited() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if time.Now().Before(g.cooldownEnd) {
		return
	}

	g.cooldownEnd = time.Now().Add(websubRateLimitCooldown)
	logger.Warnf("Websub hub rate limited us, holding off for %s", websubRateLimitCooldown)
}

func (p *Plugin) websubRequest(ytChannelID, mode string) error {
	if !websubGate.reserve() {
		return errHubCoolingDown
	}

	values := url.Values{
		"hub.callback":     {"https://" + common.ConfHost.GetString() + "/yt_new_upload/" + confWebsubVerifytoken.GetString()},
		"hub.topic":        {"https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + ytChannelID},
		"hub.verify":       {"async"},
		"hub.mode":         {mode},
		"hub.verify_token": {confWebsubVerifytoken.GetString()},
	}

	resp, err := websubClient.PostForm(GoogleWebsubHub, values)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
		websubGate.rateLimited()
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("bad status code: %d (%s) %s", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

func (p *Plugin) WebSubSubscribe(ytChannelID string) error {
	// before the post, not after: the verification callback has to get the last word on
	// the score, and it can land while the post is still in flight
	recheckAt := time.Now().Add(websubVerifyGracePeriod).Unix()
	err := common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", RedisKeyWebSubChannels, recheckAt, ytChannelID))
	if err != nil {
		logger.WithError(err).WithField("yt_channel", ytChannelID).Error("Failed deferring websub recheck")
	}

	if err := p.websubRequest(ytChannelID, "subscribe"); err != nil {
		return err
	}

	// async verification, so the hub has only accepted the request at this point
	logger.Info("Websub: Requested subscription to channel ", ytChannelID)
	return nil
}

func (p *Plugin) WebSubUnsubscribe(ytChannelID string) error {
	if err := p.websubRequest(ytChannelID, "unsubscribe"); err != nil {
		return err
	}

	logger.Info("Websub: Unsubscribed from channel ", ytChannelID)
	return nil
}

type XMLFeed struct {
	Xmlns        string `xml:"xmlns,attr"`
	Link         []Link `xml:"link"`
	ChannelID    string `xml:"entry>channelId"`
	Published    string `xml:"entry>published"`
	VideoId      string `xml:"entry>videoId"`
	Yt           string `xml:"yt,attr"`
	LinkEntry    Link   `xml:"entry>link"`
	AuthorUri    string `xml:"entry>author>uri"`
	AuthorName   string `xml:"entry>author>name"`
	UpdatedEntry string `xml:"entry>updated"`
	Title        string `xml:"title"`
	TitleEntry   string `xml:"entry>title"`
	Id           string `xml:"entry>id"`
	Updated      string `xml:"updated"`
}

type Link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}
type LinkEntry struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

const (
	GuildMaxFeeds               = 300
	GuildMaxEnabledFeeds        = 10
	GuildMaxEnabledFeedsPremium = 250
)

func MaxFeedsEnabledForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return GuildMaxEnabledFeedsPremium
	}
	return GuildMaxEnabledFeeds
}
