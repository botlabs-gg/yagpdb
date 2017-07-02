package youtube

//go:generate esc -o assets_gen.go -pkg youtube -ignore ".go" assets/

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs"
	"google.golang.org/api/youtube/v3"
	"sync"
	"time"
)

const (
	RedisChannelsLockKey = "youtube_subbed_channel_lock"
)

func KeyLastVidTime(channel string) string { return "youtube_last_video_time:" + channel }
func KeyLastVidID(channel string) string   { return "youtube_last_video_id:" + channel }

type Plugin struct {
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

	common.RegisterPlugin(p)

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
