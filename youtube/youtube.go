package youtube

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/jonas747/yagpdb/web"
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

	common.SQL.AutoMigrate(ChannelSubscription{}, YoutubePlaylistID{})

	web.RegisterPlugin(p)
	feeds.RegisterPlugin(p)
	bot.RegisterPlugin(p)
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
