package moderation

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/lib/pq"
	"github.com/mediocregopher/radix.v2/redis"
	"strconv"
	"time"
)

type Config struct {
	configstore.GuildConfigModel

	// Kick command
	KickEnabled          bool
	DeleteMessagesOnKick bool
	KickReasonOptional   bool
	KickMessage          string `valid:"template,1900"`

	// Ban
	BanEnabled        bool
	BanReasonOptional bool
	BanMessage        string `valid:"template,1900"`

	// Mute/unmute
	MuteEnabled          bool
	MuteRole             string `valid:"role,true"`
	MuteReasonOptional   bool
	UnmuteReasonOptional bool
	MuteManageRole       bool
	MuteRemoveRoles      pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	MuteIgnoreChannels   pq.Int64Array `gorm:"type:bigint[]" valid:"channel,true"`

	// Warn
	WarnCommandsEnabled    bool
	WarnIncludeChannelLogs bool
	WarnSendToModlog       bool

	// Misc
	CleanEnabled  bool
	ReportEnabled bool
	ActionChannel string `valid:"channel,true"`
	ReportChannel string `valid:"channel,true"`
	LogUnbans     bool
	LogBans       bool
}

func (c *Config) IntMuteRole() (r int64) {
	r, _ = strconv.ParseInt(c.MuteRole, 10, 64)
	return
}

func (c *Config) IntActionChannel() (r int64) {
	r, _ = strconv.ParseInt(c.ActionChannel, 10, 64)
	return
}

func (c *Config) IntReportChannel() (r int64) {
	r, _ = strconv.ParseInt(c.ReportChannel, 10, 64)
	return
}

func (c *Config) GetName() string {
	return "moderation"
}

func (c *Config) TableName() string {
	return "moderation_configs"
}

func (c *Config) Save(client *redis.Client, guildID int64) error {
	c.GuildID = guildID
	err := configstore.SQL.SetGuildConfig(context.Background(), c)
	if err != nil {
		return err
	}

	pubsub.Publish(client, "mod_refresh_mute_override", guildID, nil)
	return nil
}

type WarningModel struct {
	common.SmallModel
	GuildID  int64 `gorm:"index"`
	UserID   string
	AuthorID string

	// Username and discrim for author incase he/she leaves
	AuthorUsernameDiscrim string

	Message  string
	LogsLink string
}

func (w *WarningModel) TableName() string {
	return "moderation_warnings"
}

type MuteModel struct {
	common.SmallModel

	ExpiresAt time.Time

	GuildID int64 `gorm:"index"`
	UserID  int64

	AuthorID int64
	Reason   string

	RemovedRoles pq.Int64Array `gorm:"type:bigint[]"`
}

func (m *MuteModel) TableName() string {
	return "muted_users"
}
