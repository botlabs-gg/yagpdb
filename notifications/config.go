package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/RhykerWells/yagpdb/v2/common/pubsub"
	"github.com/RhykerWells/yagpdb/v2/lib/discordgo"
	"github.com/RhykerWells/yagpdb/v2/notifications/models"
	"github.com/RhykerWells/yagpdb/v2/web"
	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
)

func SaveConfig(config *Config) error {
	err := config.ToModel().UpsertG(context.Background(), true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	if err != nil {
		return err
	}
	pubsub.Publish("invalidate_notifications_config_cache", config.GuildID, nil)
	return nil
}

func FetchConfig(guildID int64) (*Config, error) {
	conf, err := models.FindGeneralNotificationConfigG(context.Background(), guildID)
	if err == nil {
		return configFromModel(conf), nil
	}

	if err == sql.ErrNoRows {
		return &Config{
			JoinServerMsgs: []string{"<@{{.User.ID}}> Joined!"},
			LeaveMsgs:      []string{"**{{.User.Username}}** Left... :'("},
		}, nil
	}

	return nil, err
}

// For legacy reasons, many columns in the database schema are marked as
// nullable when they should really be non-nullable, meaning working with
// models.GeneralNotificationConfig directly is much more annoying than it
// should be. We therefore wrap it in a Config (which has proper types) and
// convert to/from only when strictly required.

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

var _ web.CustomValidator = (*Config)(nil)

const MaxResponses = 10

func (c *Config) Validate(tmpl web.TemplateData, _ int64) bool {
	if len(c.JoinServerMsgs) > MaxResponses {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Too many join server messages, max %d", MaxResponses)))
		return false
	}
	if len(c.LeaveMsgs) > MaxResponses {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Too many leave server messages, max %d", MaxResponses)))
		return false
	}
	return true
}

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
