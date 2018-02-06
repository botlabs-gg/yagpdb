package youtube

//go:generate esc -o assets_gen.go -pkg youtube -ignore ".go" assets/

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/docs"
	"google.golang.org/api/youtube/v3"
	"sync"
	"time"
)

const (
	RedisChannelsLockKey = "youtube_subbed_channel_lock"
	GuildMaxFeeds        = 50
)

func KeyLastVidTime(channel string) string { return "youtube_last_video_time:" + channel }
func KeyLastVidID(channel string) string   { return "youtube_last_video_id:" + channel }

type Plugin struct {
	common.BasePlugin
	YTService *youtube.Service
	Stop      chan *sync.WaitGroup
}

func (p *Plugin) Name() string {
	return "Youtube"
}

func RegisterPlugin() {
	p := &Plugin{}
	err := p.SetupClient()
	if err != nil {
		logrus.WithError(err).Error("Failed setting up youtube plugin, youtube plugin will not be enabled.")
		return
	}

	common.GORM.AutoMigrate(ChannelSubscription{}, YoutubePlaylistID{})

	common.RegisterPluginL(p)
	mqueue.RegisterSource("youtube", p)

	docs.AddPage("Youtube Feeds", FSMustString(false, "/assets/help-page.md"), nil)
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
	if code != discordgo.ErrCodeUnknownChannel {
		logrus.WithError(err).WithField("channel", elem.Channel).Error("Error posting youtube message")
		return
	}

	// Remove it
	err = common.GORM.Where("channel_id = ?", elem.Channel).Delete(ChannelSubscription{}).Error
	if err != nil {
		p.Entry.WithError(err).Error("failed removing nonexistant channel")
	} else {
		logrus.WithField("channel", elem.Channel).Info("Removed youtube feed to nonexistant channel")
	}
}
