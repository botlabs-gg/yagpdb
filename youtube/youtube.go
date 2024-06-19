package youtube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/youtube/models"
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

func (p *Plugin) WebSubSubscribe(ytChannelID string) error {
	values := url.Values{
		"hub.callback":     {"https://" + common.ConfHost.GetString() + "/yt_new_upload/" + confWebsubVerifytoken.GetString()},
		"hub.topic":        {"https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + ytChannelID},
		"hub.verify":       {"sync"},
		"hub.mode":         {"subscribe"},
		"hub.verify_token": {confWebsubVerifytoken.GetString()},
	}

	resp, err := http.PostForm(GoogleWebsubHub, values)
	if err != nil {
		logger.WithError(err).Errorf("Failed to subscribe to youtube channel with id %s", ytChannelID)
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bad status code: %d (%s) %s", resp.StatusCode, resp.Status, string(body))
	}

	logger.Info("Websub: Subscribed to channel ", ytChannelID)
	return nil
}

func (p *Plugin) WebSubUnsubscribe(ytChannelID string) error {
	values := url.Values{
		"hub.callback":     {"https://" + common.ConfHost.GetString() + "/yt_new_upload/" + confWebsubVerifytoken.GetString()},
		"hub.topic":        {"https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + ytChannelID},
		"hub.verify":       {"sync"},
		"hub.mode":         {"unsubscribe"},
		"hub.verify_token": {confWebsubVerifytoken.GetString()},
	}

	resp, err := http.PostForm(GoogleWebsubHub, values)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("bad status code: %d (%s)", resp.StatusCode, resp.Status)
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
	GuildMaxFeeds        = 50
	GuildMaxFeedsPremium = 250
)

func MaxFeedsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return GuildMaxFeedsPremium
	}

	return GuildMaxFeeds
}
