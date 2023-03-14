package moderation

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/configstore"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/lib/pq"
)

type Config struct {
	configstore.GuildConfigModel

	// Kick command
	KickEnabled          bool
	KickCmdRoles         pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	DeleteMessagesOnKick bool
	KickReasonOptional   bool
	KickMessage          string `valid:"template,5000"`

	// Ban
	BanEnabled           bool
	BanCmdRoles          pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	BanReasonOptional    bool
	BanMessage           string        `valid:"template,5000"`
	DefaultBanDeleteDays sql.NullInt64 `gorm:"default:1" valid:"0,7"`

	// Timeout
	TimeoutEnabled              bool
	TimeoutCmdRoles             pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	TimeoutReasonOptional       bool
	TimeoutRemoveReasonOptional bool
	TimeoutMessage              string        `valid:"template,5000"`
	DefaultTimeoutDuration      sql.NullInt64 `gorm:"default:10" valid:"1,40320"`

	// Mute/unmute
	MuteEnabled             bool
	MuteCmdRoles            pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	MuteRole                string        `valid:"role,true"`
	MuteDisallowReactionAdd bool
	MuteReasonOptional      bool
	UnmuteReasonOptional    bool
	MuteManageRole          bool
	MuteRemoveRoles         pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	MuteIgnoreChannels      pq.Int64Array `gorm:"type:bigint[]" valid:"channel,true"`
	MuteMessage             string        `valid:"template,5000"`
	UnmuteMessage           string        `valid:"template,5000"`
	DefaultMuteDuration     sql.NullInt64 `gorm:"default:10" valid:"0,"`

	// Warn
	WarnCommandsEnabled    bool
	WarnCmdRoles           pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
	WarnIncludeChannelLogs bool
	WarnSendToModlog       bool
	WarnMessage            string `valid:"template,5000"`

	// Misc
	CleanEnabled  bool
	ReportEnabled bool
	ActionChannel string `valid:"channel,true"`
	ReportChannel string `valid:"channel,true"`
	ErrorChannel  string `valid:"channel,true"`
	LogUnbans     bool
	LogBans       bool
	LogKicks      bool `gorm:"default:true"`
	LogTimeouts   bool

	GiveRoleCmdEnabled bool
	GiveRoleCmdModlog  bool
	GiveRoleCmdRoles   pq.Int64Array `gorm:"type:bigint[]" valid:"role,true"`
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

func (c *Config) IntErrorChannel() (r int64) {
	r, _ = strconv.ParseInt(c.ErrorChannel, 10, 64)
	return
}

func (c *Config) GetName() string {
	return "moderation"
}

func (c *Config) TableName() string {
	return "moderation_configs"
}

func (c *Config) Save(guildID int64) error {
	c.GuildID = guildID
	err := configstore.SQL.SetGuildConfig(context.Background(), c)
	if err != nil {
		return err
	}

	if err = featureflags.UpdatePluginFeatureFlags(guildID, &Plugin{}); err != nil {
		return err
	}

	pubsub.Publish("mod_refresh_mute_override", guildID, nil)
	return err
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
