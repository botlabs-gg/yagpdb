package youtube

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"google.golang.org/api/youtube/v3"
)

const (
	RedisChannelsLockKey = "youtube_subbed_channel_lock"

	RedisKeyWebSubChannels = "youtube_registered_websub_channels"
	GoogleWebsubHub        = "https://pubsubhubbub.appspot.com/subscribe"
)

var (
	confWebsubVerifytoken = config.RegisterOption("yagpdb.youtube.verify_token", "Youtube websub push verify token, set it to a random string and never change it", "asdkpoasdkpaoksdpako")

	logger = common.GetPluginLogger(&Plugin{})
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

	common.GORM.AutoMigrate(ChannelSubscription{}, YoutubePlaylistID{})

	mqueue.RegisterSource("youtube", p)

	err := p.SetupClient()
	if err != nil {
		logger.WithError(err).Error("Failed setting up youtube plugin, youtube plugin will not be enabled.")
		return
	}
	common.RegisterPlugin(p)
}

type ChannelSubscription struct {
	common.SmallModel
	GuildID            string
	ChannelID          string
	YoutubeChannelID   string
	YoutubeChannelName string
	MentionEveryone    bool
	PublishLivestream  bool
	Enabled            bool `sql:"DEFAULT:true"`
}

func (c *ChannelSubscription) TableName() string {
	return "youtube_channel_subscriptions"
}

type YoutubePlaylistID struct {
	ChannelID  string `gorm:"primary_key"`
	CreatedAt  time.Time
	PlaylistID string
}

var _ mqueue.PluginWithSourceDisabler = (*Plugin)(nil)

// Remove feeds if they don't point to a proper channel
func (p *Plugin) DisableFeed(elem *mqueue.QueuedElement, err error) {
	// Remove it
	err = common.GORM.Where("channel_id = ?", elem.ChannelID).Delete(ChannelSubscription{}).Error
	if err != nil {
		logger.WithError(err).Error("failed removing nonexistant channel")
	} else {
		logger.WithField("channel", elem.ChannelID).Info("Removed youtube feed to nonexistant channel")
	}
}

func (p *Plugin) WebSubSubscribe(ytChannelID string) error {
	// hub.callback:https://testing.yagpdb.xyz/yt_new_upload
	// hub.topic:https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCt-ERbX-2yA6cAqfdKOlUwQ
	// hub.verify:sync
	// hub.mode:subscribe
	// hub.verify_token:hmmmmmmmmwhatsthis
	// hub.secret:
	// hub.lease_seconds:

	values := url.Values{
		"hub.callback":     {"https://" + common.ConfHost.GetString() + "/yt_new_upload/" + confWebsubVerifytoken.GetString()},
		"hub.topic":        {"https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + ytChannelID},
		"hub.verify":       {"sync"},
		"hub.mode":         {"subscribe"},
		"hub.verify_token": {confWebsubVerifytoken.GetString()},
		// "hub.lease_seconds": {"60"},
	}

	resp, err := http.PostForm(GoogleWebsubHub, values)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("go bad status code: %d (%s) %s", resp.StatusCode, resp.Status, string(body))
	}

	logger.Info("Websub: Subscribed to channel ", ytChannelID)

	return nil
}

func (p *Plugin) WebSubUnsubscribe(ytChannelID string) error {
	// hub.callback:https://testing.yagpdb.xyz/yt_new_upload
	// hub.topic:https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCt-ERbX-2yA6cAqfdKOlUwQ
	// hub.verify:sync
	// hub.mode:subscribe
	// hub.verify_token:hmmmmmmmmwhatsthis
	// hub.secret:
	// hub.lease_seconds:

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
		return fmt.Errorf("go bad status code: %d (%s)", resp.StatusCode, resp.Status)
	}

	logger.Info("Websub: UnSubscribed to channel ", ytChannelID)

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
