package notifications

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"golang.org/x/net/context"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	bot.RegisterPlugin(plugin)
	web.RegisterPlugin(plugin)

	common.SQL.AutoMigrate(&Config{})
	configstore.RegisterConfig(configstore.SQL, &Config{})

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
	configstore.GuildConfigModel
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
}

func (c *Config) GetName() string {
	return "general_notifications"
}

func (c *Config) TableName() string {
	return "general_notification_configs"
}

var DefaultConfig = &Config{}

func GetConfig(guildID string) *Config {
	var conf Config
	err := configstore.Cached.GetGuildConfig(context.Background(), guildID, &conf)
	if err != nil {
		if err != configstore.ErrNotFound {
			log.WithError(err).Error("Failed retrieving config")
		}
		return &Config{
			JoinServerMsg: "<@{{.User.ID}}> Joined!",
			LeaveMsg:      "**{{.User.Username}}** Left... :'(",
		}
	}
	return &conf
}
