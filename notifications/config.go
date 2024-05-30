package notifications

import (
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/notifications/models"
	"github.com/volatiletech/null/v8"
)

// For legacy reasons, the JoinServerMsgs and LeaveMsgs columns are not stored
// as real TEXT[] columns in database but rather single TEXT columns separated
// by U+001E (INFORMATION SEPARATOR TWO.)

const RecordSeparator = "\x1e"

func readLegacyMultiResponseColumn(s string) []string {
	return strings.Split(s, RecordSeparator)
}

func writeLegacyMultiResponseColumn(responses []string) string {
	return strings.Join(responses, RecordSeparator)
}

type Config struct {
	GuildID   int64
	CreatedAt time.Time
	UpdatedAt time.Time

	JoinServerEnabled bool  `json:"join_server_enabled" schema:"join_server_enabled"`
	JoinServerChannel int64 `json:"join_server_channel" schema:"join_server_channel" valid:"channel,true"`

	JoinServerMsgs []string `json:"join_server_msgs" schema:"join_server_msgs" valid:"template,5000"`
	JoinDMEnabled  bool     `json:"join_dm_enabled" schema:"join_dm_enabled"`
	JoinDMMsg      string   `json:"join_dm_msg" schema:"join_dm_msg" valid:"template,5000"`

	LeaveEnabled bool     `json:"leave_enabled" schema:"leave_enabled"`
	LeaveChannel int64    `json:"leave_channel" schema:"leave_channel" valid:"channel,true"`
	LeaveMsgs    []string `json:"leave_msgs" schema:"leave_msgs" valid:"template,5000"`

	TopicEnabled bool  `json:"topic_enabled" schema:"topic_enabled"`
	TopicChannel int64 `json:"topic_channel" schema:"topic_channel" valid:"channel,true"`

	CensorInvites bool `schema:"censor_invites"`
}

func (c *Config) ToModel() *models.GeneralNotificationConfig {
	return &models.GeneralNotificationConfig{
		GuildID:   c.GuildID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,

		JoinServerEnabled: null.BoolFrom(c.JoinServerEnabled),
		JoinServerChannel: null.StringFrom(discordgo.StrID(c.JoinServerChannel)),

		JoinServerMsgs: null.StringFrom(writeLegacyMultiResponseColumn(c.JoinServerMsgs)),
		JoinDMEnabled:  null.BoolFrom(c.JoinDMEnabled),
		JoinDMMsg:      null.StringFrom(c.JoinDMMsg),

		LeaveEnabled: null.BoolFrom(c.LeaveEnabled),
		LeaveChannel: null.StringFrom(discordgo.StrID(c.LeaveChannel)),
		LeaveMsgs:    null.StringFrom(writeLegacyMultiResponseColumn(c.LeaveMsgs)),

		TopicEnabled: null.BoolFrom(c.TopicEnabled),
		TopicChannel: null.StringFrom(discordgo.StrID(c.TopicChannel)),

		CensorInvites: null.BoolFrom(c.CensorInvites),
	}
}

func configFromModel(model *models.GeneralNotificationConfig) *Config {
	joinServerChannel, _ := discordgo.ParseID(model.JoinServerChannel.String)
	leaveChannel, _ := discordgo.ParseID(model.LeaveChannel.String)
	topicChannel, _ := discordgo.ParseID(model.TopicChannel.String)
	return &Config{
		GuildID:   model.GuildID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,

		JoinServerEnabled: model.JoinServerEnabled.Bool,
		JoinServerChannel: joinServerChannel,

		JoinServerMsgs: readLegacyMultiResponseColumn(model.JoinServerMsgs.String),
		JoinDMEnabled:  model.JoinDMEnabled.Bool,
		JoinDMMsg:      model.JoinDMMsg.String,

		LeaveEnabled: model.LeaveEnabled.Bool,
		LeaveChannel: leaveChannel,
		LeaveMsgs:    readLegacyMultiResponseColumn(model.LeaveMsgs.String),

		TopicEnabled: model.TopicEnabled.Bool,
		TopicChannel: topicChannel,

		CensorInvites: model.CensorInvites.Bool,
	}
}
