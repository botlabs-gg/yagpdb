package moderation

import (
	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

//go:generate sqlboiler --no-hooks psql

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

func RedisKeyLockedMute(guildID, userID int64) string {
	return "moderation_updating_mute:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)

	common.InitSchemas("moderation", DBSchemas...)
}

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagMuteRoleManaged = "moderation_mute_role_managed"
	featureFlagMuteEnabled     = "moderation_mute_enabled"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	config, err := BotCachedGetConfig(guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	var flags []string
	if config.MuteRole != 0 && config.MuteManageRole {
		flags = append(flags, featureFlagMuteRoleManaged)
	}

	if config.MuteRole != 0 {
		flags = append(flags, featureFlagMuteEnabled)
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagMuteRoleManaged, // set if this server has a valid mute role and it's managed
		featureFlagMuteEnabled,     // set if this server has a valid mute role and it's managed
	}
}
