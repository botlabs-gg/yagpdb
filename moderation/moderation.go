package moderation

import (
	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/featureflags"
	"golang.org/x/net/context"
)

const (
	ActionMuted    = "Muted"
	ActionUnMuted  = "Unmuted"
	ActionKicked   = "Kicked"
	ActionBanned   = "Banned"
	ActionUnbanned = "Unbanned"
	ActionWarned   = "Warned"
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Moderation",
		SysName:  "moderation",
		Category: common.PluginCategoryModeration,
	}
}

func RedisKeyMutedUser(guildID, userID int64) string {
	return "moderation_muted_user:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}

func RedisKeyBannedUser(guildID, userID int64) string {
	return "moderation_banned_user:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}

func RedisKeyUnbannedUser(guildID, userID int64) string {
	return "moderation_unbanned_user:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}

func RedisKeyLockedMute(guildID, userID int64) string {
	return "moderation_updating_mute:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}

func RegisterPlugin() {
	plugin := &Plugin{}

	common.RegisterPlugin(plugin)

	configstore.RegisterConfig(configstore.SQL, &Config{})
	common.GORM.AutoMigrate(&Config{}, &WarningModel{}, &MuteModel{})
}

func getConfigIfNotSet(guildID int64, config *Config) (*Config, error) {
	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

func GetConfig(guildID int64) (*Config, error) {
	var config Config
	err := configstore.Cached.GetGuildConfig(context.Background(), guildID, &config)
	if err == configstore.ErrNotFound {
		err = nil
	}
	return &config, err
}

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagMuteRoleManaged = "moderation_mute_role_managed"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	config, err := GetConfig(guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	var flags []string
	if config.MuteRole != "" && config.MuteManageRole {
		flags = append(flags, featureFlagMuteRoleManaged)
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagMuteRoleManaged, // set if this server has a valid mute role and it's managed
	}
}
