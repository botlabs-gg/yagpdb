package youtube

import (
	"context"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/premium"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/youtube/v3"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

const (
	RedisChannelsLockKey = "youtube_subbed_channel_lock"

	RedisKeyWebSubChannels = "youtube_registered_websub_channels"
	GoogleWebsubHub        = "https://pubsubhubbub.appspot.com/subscribe"
)

var (
	WebSubVerifyToken = os.Getenv("YAGPDB_YOUTUBE_VERIFY_TOKEN")
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
	err := p.SetupClient()
	if err != nil {
		logrus.WithError(err).Error("Failed setting up youtube plugin, youtube plugin will not be enabled.")
		return
	}

	common.GORM.AutoMigrate(ChannelSubscription{}, YoutubePlaylistID{})

	common.RegisterPlugin(p)
	mqueue.RegisterSource("youtube", p)
}

type ChannelSubscription struct {
	common.SmallModel
	GuildID            string
	ChannelID          string
	YoutubeChannelID   string
	YoutubeChannelName string
	MentionEveryone    bool
}

func (c *ChannelSubscription) TableName() string {
	return "youtube_channel_subscriptions"
}

type YoutubePlaylistID struct {
	ChannelID  string `gorm:"primary_key"`
	CreatedAt  time.Time
	PlaylistID string
}

// Remove feeds if they don't point to a proper channel
func (p *Plugin) HandleMQueueError(elem *mqueue.QueuedElement, err error) {
	code, _ := common.DiscordError(err)
	if code != discordgo.ErrCodeUnknownChannel && code != discordgo.ErrCodeMissingAccess && code != discordgo.ErrCodeMissingPermissions {
		logrus.WithError(err).WithField("channel", elem.Channel).Warn("Error posting youtube message")
		return
	}

	// Remove it
	err = common.GORM.Where("channel_id = ?", elem.Channel).Delete(ChannelSubscription{}).Error
	if err != nil {
		logrus.WithError(err).Error("failed removing nonexistant channel")
	} else {
		logrus.WithField("channel", elem.Channel).Info("Removed youtube feed to nonexistant channel")
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
		"hub.callback":     {"https://" + common.Conf.Host + "/yt_new_upload/" + WebSubVerifyToken},
		"hub.topic":        {"https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + ytChannelID},
		"hub.verify":       {"sync"},
		"hub.mode":         {"subscribe"},
		"hub.verify_token": {WebSubVerifyToken},
		// "hub.lease_seconds": {"60"},
	}

	resp, err := http.PostForm(GoogleWebsubHub, values)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Go bad status code: %d (%s)", resp.StatusCode, resp.Status)
	}

	logrus.Info("Websub: Subscribed to channel ", ytChannelID)

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
		"hub.callback":     {"https://" + common.Conf.Host + "/yt_new_upload/" + WebSubVerifyToken},
		"hub.topic":        {"https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + ytChannelID},
		"hub.verify":       {"sync"},
		"hub.mode":         {"unsubscribe"},
		"hub.verify_token": {WebSubVerifyToken},
	}

	resp, err := http.PostForm(GoogleWebsubHub, values)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Go bad status code: %d (%s)", resp.StatusCode, resp.Status)
	}

	logrus.Info("Websub: UnSubscribed to channel ", ytChannelID)

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
