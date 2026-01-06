package customcommands

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const (
	roleTriggerCooldownSecondsPremium = 60
	roleTriggerCooldownSeconds        = 300
)

func keyRoleTriggerCooldown(guildID, userID, roleID int64) string {
	return fmt.Sprintf("custom_command_role_trigger_cd:%d:%d:%d", guildID, userID, roleID)
}

func checkRoleTriggerCooldown(guildID, userID, roleID int64) (bool, error) {
	var exists int
	err := common.RedisPool.Do(radix.Cmd(&exists, "EXISTS", keyRoleTriggerCooldown(guildID, userID, roleID)))
	if err != nil {
		return false, err
	}
	return exists == 0, nil
}

func setRoleTriggerCooldown(guildID, userID, roleID int64, duration int) error {
	return common.RedisPool.Do(radix.FlatCmd(nil, "SETEX", keyRoleTriggerCooldown(guildID, userID, roleID), duration, "1"))
}

var cachedCommandsRoleTrigger = common.CacheSet.RegisterSlot("custom_commands_role_trigger", nil, int64(0))

func BotCachedGetCommandsWithRoleTrigger(guildID int64, ctx context.Context) ([]*models.CustomCommand, error) {
	v, err := cachedCommandsRoleTrigger.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		var cmds []*models.CustomCommand
		var err error

		common.LogLongCallTime(time.Second, true, "Took longer than a second to fetch custom commands from db", logrus.Fields{"guild": guildID}, func() {
			cmds, err = models.CustomCommands(qm.Where("guild_id = ? AND trigger_type = 11", guildID), qm.OrderBy("local_id desc"), qm.Load("Group")).AllG(ctx)
		})

		return cmds, err
	})

	if err != nil {
		return nil, err
	}

	return v.([]*models.CustomCommand), nil
}

func CmdRunsForRole(cc *models.CustomCommand, role *discordgo.Role) bool {
	if cc.GroupID.Valid {
		// check group restrictions
		if slices.Contains(cc.R.Group.IgnoreRoles, role.ID) {
			return false
		}

		if len(cc.R.Group.WhitelistRoles) > 0 && !slices.Contains(cc.R.Group.WhitelistRoles, role.ID) {
			return false
		}
	}

	// check command specific restrictions
	if len(cc.Roles) == 0 {
		// Fast path
		return !cc.RolesWhitelistMode
	}

	if slices.Contains(cc.Roles, role.ID) {
		return cc.RolesWhitelistMode
	}

	return !cc.RolesWhitelistMode
}

func findRoleTriggerCommands(ctx context.Context, guildID int64) (matches []*TriggeredCC, err error) {
	allCmds, err := BotCachedGetCommandsWithRoleTrigger(guildID, ctx)
	if err != nil {
		return nil, err
	}

	matches = make([]*TriggeredCC, 0, len(allCmds))

	for _, cmd := range allCmds {
		if cmd.Disabled || cmd.R.Group != nil && cmd.R.Group.Disabled || cmd.ContextChannel == 0 {
			continue
		}

		matches = append(matches, &TriggeredCC{
			CC: cmd,
		})
	}

	return
}

func handleGuildAuditLogEntryCreate(evt *eventsystem.EventData) {
	data := evt.GuildAuditLogEntryCreate()

	if data.ActionType == nil || *data.ActionType != discordgo.AuditLogActionMemberRoleUpdate {
		return
	}

	if !evt.HasFeatureFlag(featureFlagHasCommands) {
		return
	}

	commands, err := findRoleTriggerCommands(evt.Context(), data.GuildID)
	if err != nil {
		logger.WithField("guild", data.GuildID).WithError(err).Warn("failed fetching role trigger commands")
		return
	}

	if len(commands) == 0 {
		return
	}

	var roleChanges []struct {
		roleID int64
		added  bool
	}

	for _, change := range data.Changes {
		if change.Key == nil {
			continue
		}

		if *change.Key != discordgo.AuditLogChangeKeyRoleAdd && *change.Key != discordgo.AuditLogChangeKeyRoleRemove {
			continue
		}

		isRoleAdded := *change.Key == discordgo.AuditLogChangeKeyRoleAdd
		var roles []map[string]any
		if change.NewValue != nil {
			if roleArray, ok := change.NewValue.([]any); ok {
				for _, r := range roleArray {
					if roleMap, ok := r.(map[string]any); ok {
						roles = append(roles, roleMap)
					}
				}
			}
		}
		for _, roleMap := range roles {
			if roleIDStr, ok := roleMap["id"].(string); ok {
				if roleID, err := strconv.ParseInt(roleIDStr, 10, 64); err == nil {
					roleChanges = append(roleChanges, struct {
						roleID int64
						added  bool
					}{roleID: roleID, added: isRoleAdded})
				}
			}
		}
	}

	if len(roleChanges) == 0 {
		return
	}

	targetUserID := data.TargetID
	modUserID := data.UserID

	targetMember, err := bot.GetMember(data.GuildID, targetUserID)
	if err != nil {
		logger.WithField("guild", data.GuildID).WithField("TargetUserID", targetUserID).WithError(err).Warn("failed getting target member for role trigger")
		return
	}

	if targetMember.User.Bot {
		return
	}

	modMember, err := bot.GetMember(data.GuildID, modUserID)
	if err != nil {
		logger.WithField("guild", data.GuildID).WithField("ModUserID", modUserID).WithError(err).Warn("failed getting mod member for role trigger")
		return
	}

	for _, roleChange := range roleChanges {
		// Check cooldown
		canTrigger, err := checkRoleTriggerCooldown(data.GuildID, targetUserID, roleChange.roleID)
		if err != nil {
			logger.WithField("guild", data.GuildID).WithField("TargetUserID", targetUserID).WithField("RoleID", roleChange.roleID).WithError(err).Warn("failed checking role trigger cooldown")
			continue
		}

		if !canTrigger {
			continue // On cooldown
		}

		if len(commands) == 0 {
			continue
		}

		// Get the role object
		gs := evt.GS
		if gs == nil {
			continue
		}

		role := gs.GetRole(roleChange.roleID)
		if role == nil {
			continue
		}

		// Execute matched commands
		metricsExecutedCommands.With(prometheus.Labels{"trigger": "role"}).Inc()
		filteredCommands := make([]*TriggeredCC, 0, len(commands))
		for _, cmd := range commands {
			cmdMode := int(cmd.CC.RoleTriggerMode)
			if cmdMode == RoleTriggerModeAdd && !roleChange.added {
				continue
			}

			if cmdMode == RoleTriggerModeRemove && roleChange.added {
				continue
			}

			if !CmdRunsForRole(cmd.CC, role) {
				continue
			}

			filteredCommands = append(filteredCommands, cmd)
		}

		if len(filteredCommands) == 0 {
			continue
		}

		cooldown := roleTriggerCooldownSeconds
		if premiumEnabled, _ := premium.IsGuildPremium(data.GuildID); premiumEnabled {
			cooldown = roleTriggerCooldownSecondsPremium
		}

		if err := setRoleTriggerCooldown(data.GuildID, targetUserID, roleChange.roleID, cooldown); err != nil {
			logger.WithField("guild", data.GuildID).WithField("TargetUserID", targetUserID).WithField("RoleID", roleChange.roleID).WithError(err).Warn("failed setting role trigger cooldown")
			continue
		}

		for _, cmd := range filteredCommands {
			cs := gs.GetChannel(cmd.CC.ContextChannel)
			if cs == nil {
				continue
			}
			// Create template context with role trigger specific variables
			tmplCtx := templates.NewContext(gs, cs, nil)
			tmplCtx.GS = gs
			tmplCtx.Data["TargetMember"] = &targetMember    // member who got the role
			tmplCtx.Data["TargetUser"] = &targetMember.User // user who got the role         // Member who assigned the role
			tmplCtx.Data["Role"] = role                     // Role that was assigned/removed
			tmplCtx.Data["Author"] = &modMember.User        // User object who assigned the role
			tmplCtx.Data["RoleAdded"] = roleChange.added
			err = ExecuteCustomCommand(cmd.CC, tmplCtx)
			if err != nil {
				logger.WithField("guild", data.GuildID).WithField("cc_id", cmd.CC.LocalID).WithError(err).Error("Error executing role trigger custom command")
			}
		}

	}
}
