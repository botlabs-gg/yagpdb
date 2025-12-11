package voiceroles

import (
	"context"
	"database/sql"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/voiceroles/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	// Use OrderSyncPreState to get the state before it's updated
	eventsystem.AddHandlerFirst(p, handleVoiceStateUpdate, eventsystem.EventVoiceStateUpdate)
}

func handleVoiceStateUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	vs := evt.VoiceStateUpdate()

	if vs.UserID == 0 {
		return false, nil
	}

	totalConfig, err := GetVoiceRolesCount(evt.Context(), vs.GuildID)
	if err != nil {
		return false, err
	}

	if totalConfig == 0 {
		return false, nil
	}

	gs := bot.State.GetGuild(vs.GuildID)
	if gs == nil {
		return false, nil
	}

	ms := bot.State.GetMember(gs.ID, vs.UserID)
	if ms == nil {
		return false, nil
	}

	oldVs := gs.GetVoiceState(vs.UserID)
	beforeChannelID := int64(0)
	if oldVs != nil {
		beforeChannelID = oldVs.ChannelID
	}

	afterChannelID := vs.ChannelID

	// User left voice completely
	if beforeChannelID != 0 && afterChannelID == 0 {
		return handleVoiceLeave(gs.ID, vs.UserID, beforeChannelID)
	}

	// User joined voice
	if beforeChannelID == 0 && afterChannelID != 0 {
		return handleVoiceJoin(gs.ID, vs.UserID, afterChannelID)
	}

	// User moved between channels
	if beforeChannelID != 0 && afterChannelID != 0 && beforeChannelID != afterChannelID {
		// Remove role from old channel
		_, err := handleVoiceLeave(gs.ID, vs.UserID, beforeChannelID)
		if err != nil {
			logger.WithError(err).WithField("guild", gs.ID).WithField("user", vs.UserID).Error("Failed removing role on channel move")
		}

		// Add role for new channel
		_, err = handleVoiceJoin(gs.ID, vs.UserID, afterChannelID)
		if err != nil {
			logger.WithError(err).WithField("guild", gs.ID).WithField("user", vs.UserID).Error("Failed adding role on channel move")
		}
	}

	return false, nil
}

func handleVoiceJoin(guildID int64, userID int64, channelID int64) (retry bool, err error) {
	ctx := context.Background()

	// Get voice role config for this channel
	config, err := models.VoiceRoles(
		qm.Where("channel_id = ? and enabled = true", channelID),
	).OneG(ctx)

	if err == sql.ErrNoRows {
		// No config for this channel, ignore
		return false, nil
	}

	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed getting voice role config")
		return false, err
	}

	// Assign the role
	err = common.BotSession.GuildMemberRoleAdd(guildID, userID, config.RoleID)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Error("Failed adding voice role")
		code, _ := common.DiscordError(err)
		// Check if it's a permissions error or role hierarchy error
		switch code {
		case discordgo.ErrCodeUnknownRole:
			logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed adding voice role (role not found), deleting config")
			DeleteVoiceRoles(ctx, config.ID)
			return false, nil
		case discordgo.ErrCodeMissingPermissions:
			logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed adding voice role (permissions issue), disabling config")
			DisableVoiceRoles(ctx, config.ID)
			return false, nil
		default:
			logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed adding voice role (permissions issue)")
			return false, err
		}
	}

	return false, nil
}

func handleVoiceLeave(guildID int64, userID int64, channelID int64) (retry bool, err error) {
	ctx := context.Background()

	// Get voice role config for this channel
	config, err := models.VoiceRoles(
		qm.Where("channel_id = ? and enabled = true", channelID),
	).OneG(ctx)

	if err == sql.ErrNoRows {
		// No config for this channel, ignore
		return false, nil
	}

	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed getting voice role config")
		return false, err
	}

	// Remove the role
	err = common.BotSession.GuildMemberRoleRemove(guildID, userID, config.RoleID)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Error("Failed removing voice role")
		code, _ := common.DiscordError(err)
		// Check if it's a permissions error or role hierarchy error
		switch code {
		case discordgo.ErrCodeUnknownRole:
			logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed removing voice role (role not found), deleting config")
			DeleteVoiceRoles(ctx, config.ID)
			return false, nil
		case discordgo.ErrCodeMissingPermissions:
			logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed removing voice role (permissions issue), disabling config")
			DisableVoiceRoles(ctx, config.ID)
			return false, nil
		default:
			logger.WithError(err).WithField("guild", guildID).WithField("user", userID).WithField("role", config.RoleID).Warn("Failed removing voice role (permissions issue)")
			return false, err
		}
	}

	return false, nil
}

// GetVoiceRoles returns all voice role configs for a guild
func GetVoiceRoles(ctx context.Context, guildID int64) (*models.VoiceRoleSlice, error) {
	configs, err := models.VoiceRoles(
		qm.Where("guild_id = ?", guildID),
		qm.OrderBy("created_at ASC"),
	).AllG(ctx)

	if err == sql.ErrNoRows {
		// No config for this channel, ignore
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &configs, nil
}

// GetVoiceRolesCount returns the count of voice role configs for a guild
func GetVoiceRolesCount(ctx context.Context, guildID int64) (int64, error) {
	count, err := models.VoiceRoles(
		qm.Where("guild_id = ?", guildID),
	).CountG(ctx)

	if err == sql.ErrNoRows {
		// No config for this channel, ignore
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return count, nil
}

// CreateVoiceRoles creates a new voice role config
func CreateVoiceRoles(ctx context.Context, guildID int64, channelID int64, roleID int64) (*models.VoiceRole, error) {
	config := &models.VoiceRole{
		GuildID:   guildID,
		ChannelID: channelID,
		RoleID:    roleID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := config.InsertG(ctx, boil.Infer())
	if err != nil {
		return nil, err
	}

	return config, nil
}

// DisableVoiceRoles disables an existing voice role config
func DisableVoiceRoles(ctx context.Context, id int64) error {
	config, err := models.FindVoiceRoleG(ctx, id)
	if err == sql.ErrNoRows {
		// No config for this channel, ignore
		return nil
	}

	if err != nil {
		return err
	}

	config.Enabled = false

	_, err = config.UpdateG(ctx, boil.Infer())
	return err
}

// DeleteVoiceRoles deletes a voice role config
func DeleteVoiceRoles(ctx context.Context, id int64) error {
	config, err := models.FindVoiceRoleG(ctx, id)
	if err == sql.ErrNoRows {
		// No config for this channel, ignore
		return nil
	}

	if err != nil {
		return err
	}

	_, err = config.DeleteG(ctx)
	return err
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	ctx := context.Background()

	configs, err := models.VoiceRoles(
		qm.Where("guild_id = ?", guildID),
		qm.OrderBy("id DESC"),
	).AllG(ctx)

	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed retrieving voice roles for limit enforcement")
		return err
	}

	if len(configs) > MaxVoiceRoles {
		excessConfigs := configs[MaxVoiceRoles:]
		for _, config := range excessConfigs {
			DisableVoiceRoles(ctx, config.ID)
		}
	}

	return nil
}
