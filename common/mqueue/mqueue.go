package mqueue

import (
	"net"
	"net/http"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/sirupsen/logrus"
)

var (
	sources        = make(map[string]PluginWithSourceDisabler)
	webhookSession *discordgo.Session
	logger         = common.GetPluginLogger(&Plugin{})
)

// PluginWithSourceDisabler
type PluginWithSourceDisabler interface {
	DisableFeed(elem *QueuedElement, err error)
}

// PluginWithWebhookAvatar can be implemented by plugins for custom avatars
type PluginWithWebhookAvatar interface {
	WebhookAvatar() string
}

var (
	_ bot.LateBotInitHandler = (*Plugin)(nil)
	_ bot.BotStopperHandler  = (*Plugin)(nil)
)

// Plugin represents the mqueue plugin
type Plugin struct {
	server *MqueueServer
}

// PluginInfo implements common.Plugin
func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "mqueue",
		SysName:  "mqueue",
		Category: common.PluginCategoryCore,
	}
}

// RegisterPlugin registers the mqueue plugin into the plugin system and also initializes it
func RegisterPlugin() {

	var err error
	webhookSession, err = discordgo.New()
	if err != nil {
		logger.WithError(err).Error("failed initiializing webhook session")
	}
	webhookSession.AddHandler(handleWebhookSessionRatelimit)

	innerTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   10,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	webhookSession.Client.Transport = innerTransport

	_, err = common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Error("[mqueue] failed initiializing db schema")
	}

	p := &Plugin{}
	common.RegisterPlugin(p)
}

// RegisterSource registers a mqueue source, used for error handling
func RegisterSource(name string, source PluginWithSourceDisabler) {
	sources[name] = source
}

type Storage interface {
	GetFullQueue() ([]*workItem, error)
	AppendItem(elem *QueuedElement) error
	DelItem(elem *workItem) error
	NextID() (int64, error)
}
