package quicksetup

import (
	"context"
	"database/sql"

	automodmodels "github.com/botlabs-gg/yagpdb/v2/automod/models"
	"github.com/botlabs-gg/yagpdb/v2/autorole"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/notifications"
	ticketmodels "github.com/botlabs-gg/yagpdb/v2/tickets/models"
	verificationmodels "github.com/botlabs-gg/yagpdb/v2/verification/models"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// A module is one feature the setup can turn on. Only modules whose plugin
// is loaded on this instance are offered.
type module struct {
	ID            string
	Name          string
	Description   string
	PluginSysName string
	SettingsPath  string
	Icon          string
	Color         string // a bootstrap color name
	RequiredPerms []int64

	alreadySetUp func(ctx context.Context, guildID int64) (bool, error)
}

var modules = []*module{
	{
		ID:            "moderation",
		Name:          "Moderation",
		Description:   "Commands to ban, kick, mute and warn people, with a log of everything your moderators do.",
		PluginSysName: "moderation",
		SettingsPath:  "/moderation",
		Icon:          "fas fa-gavel",
		Color:         "danger",
		RequiredPerms: []int64{discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionModerateMembers, discordgo.PermissionManageMessages},
		alreadySetUp:  moderationSetUp,
	},
	{
		ID:            "automod",
		Name:          "Auto Moderation",
		Description:   "Stops spam, scam links, invites to other servers and mass pings, all on its own.",
		PluginSysName: "automod_v2",
		SettingsPath:  "/automod",
		Icon:          "fas fa-shield-alt",
		Color:         "primary",
		RequiredPerms: []int64{discordgo.PermissionManageMessages, discordgo.PermissionModerateMembers},
		alreadySetUp:  automodSetUp,
	},
	{
		ID:            "welcome",
		Name:          "Welcome Messages",
		Description:   "Says hello to new members when they join.",
		PluginSysName: "notifications",
		SettingsPath:  "/notifications/general",
		Icon:          "fas fa-door-open",
		Color:         "success",
		RequiredPerms: []int64{discordgo.PermissionSendMessages},
		alreadySetUp:  welcomeSetUp,
	},
	{
		ID:            "autorole",
		Name:          "Auto Role",
		Description:   "Gives new members a role as soon as they join.",
		PluginSysName: "autorole",
		SettingsPath:  "/autorole",
		Icon:          "fas fa-user-tag",
		Color:         "info",
		RequiredPerms: []int64{discordgo.PermissionManageRoles},
		alreadySetUp:  autoroleSetUp,
	},
	{
		ID:            "tickets",
		Name:          "Tickets",
		Description:   "Lets members contact your staff privately when they need help.",
		PluginSysName: "tickets",
		SettingsPath:  "/tickets",
		Icon:          "fas fa-ticket-alt",
		Color:         "warning",
		RequiredPerms: []int64{discordgo.PermissionManageChannels},
		alreadySetUp:  ticketsSetUp,
	},
	{
		ID:            "verification",
		Name:          "Verification",
		Description:   "Makes new members prove they are human before they can chat.",
		PluginSysName: "verification",
		SettingsPath:  "/verification",
		Icon:          "fas fa-user-check",
		Color:         "success",
		RequiredPerms: []int64{discordgo.PermissionManageRoles},
		alreadySetUp:  verificationSetUp,
	},
}

func availableModules() []*module {
	var available []*module
	for _, m := range modules {
		if common.PluginLoaded(m.PluginSysName) {
			available = append(available, m)
		}
	}

	return available
}

func moduleAvailable(id string) bool {
	for _, m := range availableModules() {
		if m.ID == id {
			return true
		}
	}

	return false
}

func moderationSetUp(ctx context.Context, guildID int64) (bool, error) {
	config, err := moderation.FetchConfig(guildID)
	if err != nil {
		return false, err
	}

	return config.ActionChannel != 0 || config.BanEnabled || config.KickEnabled || config.TimeoutEnabled || config.WarnCommandsEnabled, nil
}

func automodSetUp(ctx context.Context, guildID int64) (bool, error) {
	return automodmodels.AutomodRulesets(qm.Where("guild_id = ?", guildID)).ExistsG(ctx)
}

func welcomeSetUp(ctx context.Context, guildID int64) (bool, error) {
	config, err := notifications.FetchConfig(guildID)
	if err != nil {
		return false, err
	}

	return config.JoinServerEnabled || config.JoinDMEnabled, nil
}

func autoroleSetUp(ctx context.Context, guildID int64) (bool, error) {
	config, err := autorole.GetAutoroleConfig(guildID)
	if err != nil {
		return false, err
	}

	return config.Role != 0, nil
}

func ticketsSetUp(ctx context.Context, guildID int64) (bool, error) {
	config, err := ticketmodels.FindTicketConfigG(ctx, guildID)
	if err == sql.ErrNoRows {
		return false, nil
	}

	return err == nil && config.Enabled, err
}

func verificationSetUp(ctx context.Context, guildID int64) (bool, error) {
	config, err := verificationmodels.FindVerificationConfigG(ctx, guildID)
	if err == sql.ErrNoRows {
		return false, nil
	}

	return err == nil && config.Enabled, err
}

func missingPermNames(botPerms int64, required []int64) []string {
	if botPerms&discordgo.PermissionAdministrator != 0 {
		return nil
	}

	var missing []string
	for _, perm := range required {
		if botPerms&perm != perm {
			missing = append(missing, common.StringPerms[perm])
		}
	}

	return missing
}
