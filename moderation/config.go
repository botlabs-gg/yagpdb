package moderation

import (
	"context"
	"database/sql"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/moderation/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/types"
)

func SaveConfig(config *Config) error {
	err := config.ToModel().UpsertG(context.Background(), true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	if err != nil {
		return err
	}
	pubsub.Publish("invalidate_moderation_config_cache", config.GuildID, nil)

	if err := featureflags.UpdatePluginFeatureFlags(config.GuildID, &Plugin{}); err != nil {
		return err
	}
	pubsub.Publish("mod_refresh_mute_override", config.GuildID, nil)
	return nil
}

func FetchConfig(guildID int64) (*Config, error) {
	conf, err := models.FindModerationConfigG(context.Background(), guildID)
	if err == nil {
		return configFromModel(conf), nil
	}

	if err == sql.ErrNoRows {
		return &Config{GuildID: guildID}, nil
	}

	return nil, err
}

// For legacy reasons, many columns in the database schema are marked as
// nullable when they should really be non-nullable, meaning working with
// models.ModerationConfig directly is much more annoying than it should be. We
// therefore wrap it in a Config (which has proper types) and convert to/from
// only when strictly required.

type Config struct {
	GuildID   int64
	CreatedAt time.Time
	UpdatedAt time.Time

	// Kick
	KickEnabled          bool
	KickCmdRoles         types.Int64Array `valid:"role,true"`
	DeleteMessagesOnKick bool
	KickReasonOptional   bool
	KickMessage          string `valid:"template,5000"`

	// Ban
	BanEnabled           bool
	BanCmdRoles          types.Int64Array `valid:"role,true"`
	BanReasonOptional    bool
	BanMessage           string     `valid:"template,5000"`
	DefaultBanDeleteDays null.Int64 `valid:"0,7"`

	// Timeout
	TimeoutEnabled              bool
	TimeoutCmdRoles             types.Int64Array `valid:"role,true"`
	TimeoutReasonOptional       bool
	TimeoutRemoveReasonOptional bool
	TimeoutMessage              string     `valid:"template,5000"`
	DefaultTimeoutDuration      null.Int64 `valid:"1,40320"`

	// Mute/unmute
	MuteEnabled             bool
	MuteCmdRoles            types.Int64Array `valid:"role,true"`
	MuteRole                int64            `valid:"role,true"`
	MuteDisallowReactionAdd bool
	MuteReasonOptional      bool
	UnmuteReasonOptional    bool
	MuteManageRole          bool
	MuteRemoveRoles         types.Int64Array `valid:"role,true"`
	MuteIgnoreChannels      types.Int64Array `valid:"channel,true"`
	MuteMessage             string           `valid:"template,5000"`
	UnmuteMessage           string           `valid:"template,5000"`
	DefaultMuteDuration     null.Int64       `valid:"0,"`

	// Warn
	WarnCommandsEnabled      bool
	WarnCmdRoles             types.Int64Array `valid:"role,true"`
	WarnIncludeChannelLogs   bool
	WarnSendToModlog         bool
	DelwarnSendToModlog      bool
	DelwarnIncludeWarnReason bool
	WarnMessage              string `valid:"template,5000"`

	// Misc
	CleanEnabled       bool
	ReportEnabled      bool
	ReportMentionRoles types.Int64Array `valid:"role,true"`
	ActionChannel      int64            `valid:"channel,true"`
	ReportChannel      int64            `valid:"channel,true"`
	ErrorChannel       int64            `valid:"channel,true"`
	LogUnbans          bool
	LogBans            bool
	LogKicks           bool
	LogTimeouts        bool

	GiveRoleCmdEnabled bool
	GiveRoleCmdModlog  bool
	GiveRoleCmdRoles   types.Int64Array `valid:"role,true"`
}

func (c *Config) ToModel() *models.ModerationConfig {
	return &models.ModerationConfig{
		GuildID:   c.GuildID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,

		KickEnabled:          null.BoolFrom(c.KickEnabled),
		KickCmdRoles:         c.KickCmdRoles,
		DeleteMessagesOnKick: null.BoolFrom(c.DeleteMessagesOnKick),
		KickReasonOptional:   null.BoolFrom(c.KickReasonOptional),
		KickMessage:          null.StringFrom(c.KickMessage),

		BanEnabled:           null.BoolFrom(c.BanEnabled),
		BanCmdRoles:          c.BanCmdRoles,
		BanReasonOptional:    null.BoolFrom(c.BanReasonOptional),
		BanMessage:           null.StringFrom(c.BanMessage),
		DefaultBanDeleteDays: c.DefaultBanDeleteDays,

		TimeoutEnabled:              null.BoolFrom(c.TimeoutEnabled),
		TimeoutCmdRoles:             c.TimeoutCmdRoles,
		TimeoutReasonOptional:       null.BoolFrom(c.TimeoutReasonOptional),
		TimeoutRemoveReasonOptional: null.BoolFrom(c.TimeoutRemoveReasonOptional),
		TimeoutMessage:              null.StringFrom(c.TimeoutMessage),
		DefaultTimeoutDuration:      c.DefaultTimeoutDuration,

		MuteEnabled:             null.BoolFrom(c.MuteEnabled),
		MuteCmdRoles:            c.MuteCmdRoles,
		MuteRole:                null.StringFrom(discordgo.StrID(c.MuteRole)),
		MuteDisallowReactionAdd: null.BoolFrom(c.MuteDisallowReactionAdd),
		MuteReasonOptional:      null.BoolFrom(c.MuteReasonOptional),
		UnmuteReasonOptional:    null.BoolFrom(c.UnmuteReasonOptional),
		MuteManageRole:          null.BoolFrom(c.MuteManageRole),
		MuteRemoveRoles:         c.MuteRemoveRoles,
		MuteIgnoreChannels:      c.MuteIgnoreChannels,
		MuteMessage:             null.StringFrom(c.MuteMessage),
		UnmuteMessage:           null.StringFrom(c.UnmuteMessage),
		DefaultMuteDuration:     c.DefaultMuteDuration,

		WarnCommandsEnabled:      null.BoolFrom(c.WarnCommandsEnabled),
		WarnCmdRoles:             c.WarnCmdRoles,
		WarnIncludeChannelLogs:   null.BoolFrom(c.WarnIncludeChannelLogs),
		WarnSendToModlog:         null.BoolFrom(c.WarnSendToModlog),
		DelwarnSendToModlog:      c.DelwarnSendToModlog,
		DelwarnIncludeWarnReason: c.DelwarnIncludeWarnReason,
		WarnMessage:              null.StringFrom(c.WarnMessage),

		CleanEnabled:       null.BoolFrom(c.CleanEnabled),
		ReportEnabled:      null.BoolFrom(c.ReportEnabled),
		ReportMentionRoles: c.ReportMentionRoles,
		ActionChannel:      null.StringFrom(discordgo.StrID(c.ActionChannel)),
		ReportChannel:      null.StringFrom(discordgo.StrID(c.ReportChannel)),
		ErrorChannel:       null.StringFrom(discordgo.StrID(c.ErrorChannel)),
		LogUnbans:          null.BoolFrom(c.LogUnbans),
		LogBans:            null.BoolFrom(c.LogBans),
		LogKicks:           null.BoolFrom(c.LogKicks),
		LogTimeouts:        null.BoolFrom(c.LogTimeouts),

		GiveRoleCmdEnabled: null.BoolFrom(c.GiveRoleCmdEnabled),
		GiveRoleCmdModlog:  null.BoolFrom(c.GiveRoleCmdModlog),
		GiveRoleCmdRoles:   c.GiveRoleCmdRoles,
	}
}

func configFromModel(model *models.ModerationConfig) *Config {
	muteRole, _ := discordgo.ParseID(model.MuteRole.String)
	actionChannel, _ := discordgo.ParseID(model.ActionChannel.String)
	reportChannel, _ := discordgo.ParseID(model.ReportChannel.String)
	errorChannel, _ := discordgo.ParseID(model.ErrorChannel.String)

	return &Config{
		GuildID:   model.GuildID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,

		KickEnabled:          model.KickEnabled.Bool,
		KickCmdRoles:         model.KickCmdRoles,
		DeleteMessagesOnKick: model.DeleteMessagesOnKick.Bool,
		KickReasonOptional:   model.KickReasonOptional.Bool,
		KickMessage:          model.KickMessage.String,

		BanEnabled:           model.BanEnabled.Bool,
		BanCmdRoles:          model.BanCmdRoles,
		BanReasonOptional:    model.BanReasonOptional.Bool,
		BanMessage:           model.BanMessage.String,
		DefaultBanDeleteDays: model.DefaultBanDeleteDays,

		TimeoutEnabled:              model.TimeoutEnabled.Bool,
		TimeoutCmdRoles:             model.TimeoutCmdRoles,
		TimeoutReasonOptional:       model.TimeoutReasonOptional.Bool,
		TimeoutRemoveReasonOptional: model.TimeoutRemoveReasonOptional.Bool,
		TimeoutMessage:              model.TimeoutMessage.String,
		DefaultTimeoutDuration:      model.DefaultTimeoutDuration,

		MuteEnabled:             model.MuteEnabled.Bool,
		MuteCmdRoles:            model.MuteCmdRoles,
		MuteRole:                muteRole,
		MuteDisallowReactionAdd: model.MuteDisallowReactionAdd.Bool,
		MuteReasonOptional:      model.MuteReasonOptional.Bool,
		UnmuteReasonOptional:    model.UnmuteReasonOptional.Bool,
		MuteManageRole:          model.MuteManageRole.Bool,
		MuteRemoveRoles:         model.MuteRemoveRoles,
		MuteIgnoreChannels:      model.MuteIgnoreChannels,
		MuteMessage:             model.MuteMessage.String,
		UnmuteMessage:           model.UnmuteMessage.String,
		DefaultMuteDuration:     model.DefaultMuteDuration,

		WarnCommandsEnabled:      model.WarnCommandsEnabled.Bool,
		WarnCmdRoles:             model.WarnCmdRoles,
		WarnIncludeChannelLogs:   model.WarnIncludeChannelLogs.Bool,
		WarnSendToModlog:         model.WarnSendToModlog.Bool,
		DelwarnSendToModlog:      model.DelwarnSendToModlog,
		DelwarnIncludeWarnReason: model.DelwarnIncludeWarnReason,
		WarnMessage:              model.WarnMessage.String,

		CleanEnabled:       model.CleanEnabled.Bool,
		ReportEnabled:      model.ReportEnabled.Bool,
		ReportMentionRoles: model.ReportMentionRoles,
		ActionChannel:      actionChannel,
		ReportChannel:      reportChannel,
		ErrorChannel:       errorChannel,
		LogUnbans:          model.LogUnbans.Bool,
		LogBans:            model.LogBans.Bool,
		LogKicks:           model.LogKicks.Bool,
		LogTimeouts:        model.LogTimeouts.Bool,

		GiveRoleCmdEnabled: model.GiveRoleCmdEnabled.Bool,
		GiveRoleCmdModlog:  model.GiveRoleCmdModlog.Bool,
		GiveRoleCmdRoles:   model.GiveRoleCmdRoles,
	}
}
