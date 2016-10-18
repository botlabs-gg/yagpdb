package notifications

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	bot.RegisterPlugin(plugin)
	web.RegisterPlugin(plugin)
}

func (p *Plugin) Name() string {
	return "Notifications"
}

func (p *Plugin) InitBot() {
	common.BotSession.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(bot.CustomGuildMemberAdd(HandleGuildMemberAdd))
	common.BotSession.AddHandler(bot.CustomGuildMemberRemove(HandleGuildMemberRemove))
	common.BotSession.AddHandler(bot.CustomChannelUpdate(HandleChannelUpdate))
}

type Config struct {
	JoinServerEnabled bool   `json:"join_server_enabled" schema:"join_server_enabled"`
	JoinServerChannel string `json:"join_server_channel" schema:"join_server_channel" valid:"channel,true"`
	JoinServerMsg     string `json:"join_server_msg" schema:"join_server_msg" valid:"template,2000"`

	JoinDMEnabled bool   `json:"join_dm_enabled" schema:"join_dm_enabled"`
	JoinDMMsg     string `json:"join_dm_msg" schema:"join_dm_msg" valid:"template,2000"`

	LeaveEnabled bool   `json:"leave_enabled" schema:"leave_enabled"`
	LeaveChannel string `json:"leave_channel" schema:"leave_channel" valid:"channel,true"`
	LeaveMsg     string `json:"leave_msg" schema:"leave_msg" valid:"template,500"`

	TopicEnabled bool   `json:"topic_enabled" schema:"topic_enabled"`
	TopicChannel string `json:"topic_channel" schema:"topic_channel" valid:"channel,true"`

	// Deprecated
	// Need to safely remove these fields from redis with a script
	PinEnabled bool   `json:"pin_enabled,omitempty"`
	PinChannel string `json:"pin_channel,omitempty"`
}

var DefaultConfig = &Config{}

func GetConfig(client *redis.Client, server string) *Config {
	var config *Config
	if err := common.GetRedisJson(client, "notifications/general:"+server, &config); err != nil {
		log.WithError(err).WithField("guild", server).Error("Failed retrieving noifications config")
		return DefaultConfig
	}
	if config == nil {
		return DefaultConfig
	}
	return config
}
