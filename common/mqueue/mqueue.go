package mqueue

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/jonas747/yagpdb/common/config"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

var (
	sources        = make(map[string]PluginWithSourceDisabler)
	webhookSession *discordgo.Session
	logger         = common.GetPluginLogger(&Plugin{})
	confMaxWorkers = config.RegisterOption("yagpdb.mqueue.max_workers", "Max mqueue sending workers", 2)
)

// PluginWithSourceDisabler todo
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

func incrIDCounter() (next int64) {
	err := common.RedisPool.Do(radix.Cmd(&next, "INCR", "mqueue_id_counter"))
	if err != nil {
		logger.WithError(err).Error("Failed increasing mqueue id counter")
		return -1
	}

	return next
}

// QueueMessage queues a message in the message queue
func QueueMessage(elem *QueuedElement) {
	nextID := incrIDCounter()
	if nextID == -1 {
		return
	}

	elem.ID = nextID

	serialized, err := json.Marshal(elem)
	if err != nil {
		logger.WithError(err).Error("Failed marshaling mqueue element")
		return
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "ZADD", "mqueue", "-1", string(serialized)))
	if err != nil {
		return
	}
}
