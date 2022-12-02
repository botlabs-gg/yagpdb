package notifications

import (
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/configstore"
	"golang.org/x/net/context"
)

const (
	RecordSeparator = "\x1e"
	MaxUserMessages = 10
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)

	common.GORM.AutoMigrate(&Config{})
	configstore.RegisterConfig(configstore.SQL, &Config{})

}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "General Notifications",
		SysName:  "notifications",
		Category: common.PluginCategoryFeeds,
	}
}

type Config struct {
	configstore.GuildConfigModel
	JoinServerEnabled bool   `json:"join_server_enabled" schema:"join_server_enabled"`
	JoinServerChannel string `json:"join_server_channel" schema:"join_server_channel" valid:"channel,true"`

	// Implementation note: gorilla/schema currently requires manual index
	// setting in forms to parse sub-objects. GORM has_many is also complicated
	// by manual handling of associations and loss of IDs through the web form
	// (without which Replace() is currently n^2).
	// For strings, we greatly simplify things by flattening for storage.

	// TODO: Remove the legacy single-message variant when ready to migrate the
	// database.
	JoinServerMsg  string   `json:"join_server_msg" valid:"template,5000"`
	JoinServerMsgs []string `json:"join_server_msgs" schema:"join_server_msgs" gorm:"-" valid:"template,5000"`
	// Do Not Use! For persistence only.
	JoinServerMsgs_ string `json:"-"`

	JoinDMEnabled bool   `json:"join_dm_enabled" schema:"join_dm_enabled"`
	JoinDMMsg     string `json:"join_dm_msg" schema:"join_dm_msg" valid:"template,5000"`

	LeaveEnabled bool     `json:"leave_enabled" schema:"leave_enabled"`
	LeaveChannel string   `json:"leave_channel" schema:"leave_channel" valid:"channel,true"`
	LeaveMsg     string   `json:"leave_msg" schema:"leave_msg" valid:"template,5000"`
	LeaveMsgs    []string `json:"leave_msgs" schema:"leave_msgs" gorm:"-" valid:"template,5000"`
	// Do Not Use! For persistence only.
	LeaveMsgs_ string `json:"-"`

	TopicEnabled bool   `json:"topic_enabled" schema:"topic_enabled"`
	TopicChannel string `json:"topic_channel" schema:"topic_channel" valid:"channel,true"`

	CensorInvites bool `schema:"censor_invites"`
}

func (c *Config) JoinServerChannelInt() (i int64) {
	i, _ = strconv.ParseInt(c.JoinServerChannel, 10, 64)
	return
}

func (c *Config) LeaveChannelInt() (i int64) {
	i, _ = strconv.ParseInt(c.LeaveChannel, 10, 64)
	return
}

func (c *Config) TopicChannelInt() (i int64) {
	i, _ = strconv.ParseInt(c.TopicChannel, 10, 64)
	return
}

func (c *Config) GetName() string {
	return "general_notifications"
}

func (c *Config) TableName() string {
	return "general_notification_configs"
}

// GORM BeforeSave hook
func (c *Config) BeforeSave() (err error) {
	filterAndJoin := func(a []string) string {
		joined := ""
		msgsJoined := 0
		for _, s := range a {
			if s == "" {
				continue
			}
			if msgsJoined >= MaxUserMessages {
				break
			}
			msgsJoined++

			if len(joined) > 0 {
				joined += RecordSeparator
			}

			joined += s
		}

		return joined
	}

	c.JoinServerMsgs_ = filterAndJoin(c.JoinServerMsgs)
	c.LeaveMsgs_ = filterAndJoin(c.LeaveMsgs)

	return nil
}

// GORM AfterFind hook
func (c *Config) AfterFind() (err error) {
	if c.JoinServerMsg != "" {
		c.JoinServerMsgs = append(c.JoinServerMsgs, c.JoinServerMsg)
		c.JoinServerMsg = ""
	}
	if c.JoinServerMsgs_ != "" {
		c.JoinServerMsgs = append(c.JoinServerMsgs, strings.Split(c.JoinServerMsgs_, RecordSeparator)...)
	}

	if c.LeaveMsg != "" {
		c.LeaveMsgs = append(c.LeaveMsgs, c.LeaveMsg)
		c.LeaveMsg = ""
	}
	if c.LeaveMsgs_ != "" {
		c.LeaveMsgs = append(c.LeaveMsgs, strings.Split(c.LeaveMsgs_, RecordSeparator)...)
	}

	return nil
}

var DefaultConfig = &Config{}

func GetConfig(guildID int64) (*Config, error) {
	var conf Config
	err := configstore.Cached.GetGuildConfig(context.Background(), guildID, &conf)
	if err == nil {
		return &conf, nil
	}

	if err == configstore.ErrNotFound {
		// if err != configstore.ErrNotFound {
		// 	log.WithError(err).Error("Failed retrieving config")
		// }
		return &Config{
			JoinServerMsgs: []string{"<@{{.User.ID}}> Joined!"},
			LeaveMsgs:      []string{"**{{.User.Username}}** Left... :'("},
		}, nil
	}

	return nil, errors.WithStackIf(err)
}
