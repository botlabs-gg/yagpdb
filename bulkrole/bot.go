package bulkrole

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/mediocregopher/radix/v3"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.BotStopperHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, handleGuildChunk, eventsystem.EventGuildMembersChunk)
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	wg.Done()
}

// Redis keys for bulk role operations
func RedisKeyBulkRoleStatus(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":status"
}

func RedisKeyBulkRoleMembers(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":members"
}

func RedisKeyBulkRoleProcessed(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":processed"
}

func RedisKeyBulkRoleResults(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":results"
}

// Redis key for rate limiting between bulk role operations
func RedisKeyBulkRoleCooldown(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":cooldown"
}

// Redis key to track how many chunks have been processed
func RedisKeyBulkRoleChunksProcessed(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":chunks_processed"
}

// Redis key to guard finalization (avoid duplicate notifications/cleanup)
func RedisKeyBulkRoleFinalized(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":finalized"
}

func isRateLimitActive(guildID int64) bool {
	var cooldownActive int
	common.RedisPool.Do(radix.Cmd(&cooldownActive, "EXISTS", RedisKeyBulkRoleCooldown(guildID)))
	return cooldownActive > 0
}

func getRemainingCooldown(guildID int64) int64 {
	var ttl int64
	common.RedisPool.Do(radix.Cmd(&ttl, "TTL", RedisKeyBulkRoleCooldown(guildID)))
	return ttl
}

func isBulkRoleCancelled(guildID int64) bool {
	var status int
	common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))
	return status == BulkRoleCancelled
}

// Handle guild member chunk for bulk role operations
func handleGuildChunk(evt *eventsystem.EventData) {
	chunk := evt.GuildMembersChunk()
	guildID := chunk.GuildID

	if !IsBulkRoleOperationActive(guildID) {
		return
	}

	if chunk.Nonce == "" || strconv.Itoa(int(guildID)) != chunk.Nonce {
		return
	}

	config, err := GetBulkRoleConfig(guildID)
	if err != nil {
		logger.WithError(err).Error("Failed to get bulkrole config")
		return
	}

	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterating)))
	go config.processBulkRoleChunk(chunk)
}

// Process a chunk of members for bulk role operations
func (config *BulkRoleConfig) processBulkRoleChunk(chunk *discordgo.GuildMembersChunk) {
	if err := config.canBotAssignRole(); err != nil {
		logger.WithError(err).WithField("guild", config.GuildID).Error("Bot lost permissions during bulk role operation, canceling")
		config.forceOperationCompletion("Bot lost permissions during operation")
		return
	}
	// Local per-chunk counters to avoid races across concurrent chunk goroutines
	chunkProcessed := 0
	chunkResults := 0

	lastTimeStatusRefreshed := time.Now()

	guildID := config.GuildID
	for _, member := range chunk.Members {
		if isBulkRoleCancelled(guildID) {
			return
		}

		chunkProcessed++

		if !config.filterMember(member) {
			continue
		}

		hasRole := common.ContainsInt64Slice(member.Roles, config.TargetRole)
		needsOperation := false

		switch config.Operation {
		case "assign":
			needsOperation = !hasRole
		case "remove":
			needsOperation = hasRole
		}

		if !needsOperation {
			continue
		}

		var err error
		switch config.Operation {
		case "assign":
			err = bot.ShardManager.SessionForGuild(guildID).GuildMemberRoleAdd(guildID, member.User.ID, config.TargetRole)
		case "remove":
			err = bot.ShardManager.SessionForGuild(guildID).GuildMemberRoleRemove(guildID, member.User.ID, config.TargetRole)
		}

		if err != nil {
			logger.WithError(err).WithField("guild", guildID).WithField("user", member.User.ID).Error("Failed to modify role")
			continue
		}

		chunkResults++

		// Rate limiting
		time.Sleep(time.Millisecond * 100)

		// Refresh status every 50 seconds to keep Redis keys alive
		if time.Since(lastTimeStatusRefreshed) > time.Second*50 {
			if isBulkRoleCancelled(guildID) {
				return
			}

			lastTimeStatusRefreshed = time.Now()
			err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterating)))
			if err != nil {
				logger.WithError(err).Error("Failed refreshing bulk role iterating status")
			}
		}
	}

	if chunkProcessed > 0 {
		common.RedisPool.Do(radix.Cmd(nil, "INCRBY", RedisKeyBulkRoleProcessed(guildID), strconv.Itoa(chunkProcessed)))
	}
	if chunkResults > 0 {
		common.RedisPool.Do(radix.Cmd(nil, "INCRBY", RedisKeyBulkRoleResults(guildID), strconv.Itoa(chunkResults)))
	}

	var doneChunks int
	common.RedisPool.Do(radix.Cmd(&doneChunks, "INCR", RedisKeyBulkRoleChunksProcessed(guildID)))

	if doneChunks >= chunk.ChunkCount {
		if isBulkRoleCancelled(guildID) {
			return
		}
		err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterationDone)))
		if err != nil {
			logger.WithError(err).Error("Failed marking bulk role iteration complete")
		}
		common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleStatus(guildID), strconv.Itoa(BulkRoleCompleted)))
		var finalProcessed, finalResults int
		common.RedisPool.Do(radix.Cmd(&finalProcessed, "GET", RedisKeyBulkRoleProcessed(guildID)))
		common.RedisPool.Do(radix.Cmd(&finalResults, "GET", RedisKeyBulkRoleResults(guildID)))
		config.sendNotificationAlert("Completed", finalProcessed, finalResults, "")
		common.RedisPool.Do(radix.Cmd(nil, "DEL",
			RedisKeyBulkRoleStatus(guildID),
			RedisKeyBulkRoleProcessed(guildID),
			RedisKeyBulkRoleResults(guildID),
			RedisKeyBulkRoleChunksProcessed(guildID)))

		logger.WithField("guild", guildID).WithField("processed", finalProcessed).Info("Bulk role operation completed")
	} else {
		logger.WithField("guild", guildID).WithField("doneChunks", doneChunks).WithField("chunkCount", chunk.ChunkCount).Debug("Processed chunk, waiting for more")
	}
}

// Check if a member meets the filter criteria
func (config *BulkRoleConfig) filterMember(member *discordgo.Member) bool {
	switch config.FilterType {
	case "all":
		return true
	case "has_roles":
		if len(config.FilterRoleIDs) == 0 {
			return false
		}
		if config.FilterRequireAll {
			for _, roleID := range config.FilterRoleIDs {
				if !common.ContainsInt64Slice(member.Roles, roleID) {
					return false
				}
			}
			return true
		} else {
			for _, roleID := range config.FilterRoleIDs {
				if common.ContainsInt64Slice(member.Roles, roleID) {
					return true
				}
			}
			return false
		}
	case "missing_roles":
		if len(config.FilterRoleIDs) == 0 {
			return false
		}
		if config.FilterRequireAll {
			for _, roleID := range config.FilterRoleIDs {
				if common.ContainsInt64Slice(member.Roles, roleID) {
					return false
				}
			}
			return true
		} else {
			for _, roleID := range config.FilterRoleIDs {
				if !common.ContainsInt64Slice(member.Roles, roleID) {
					return true
				}
			}
			return false
		}
	case "bots":
		return member.User.Bot
	case "humans":
		return !member.User.Bot
	case "joined_after":
		if config.FilterDateParsed.IsZero() {
			return false
		}
		joinedAt, _ := member.JoinedAt.Parse()
		return joinedAt.After(config.FilterDateParsed)
	case "joined_before":
		if config.FilterDateParsed.IsZero() {
			return false
		}
		joinedAt, _ := member.JoinedAt.Parse()
		return joinedAt.Before(config.FilterDateParsed)
	default:
		return false
	}
}

func (config *BulkRoleConfig) canBotAssignRole() error {
	// Check if target role exists and bot can manage it
	guild, err := botrest.GetGuild(config.GuildID)
	if err != nil {
		return errors.WithMessage(err, "failed to get guild")
	}

	if guild == nil {
		return errors.New("failed to get guild")
	}

	targetRole := guild.GetRole(config.TargetRole)
	if targetRole == nil {
		return errors.New("failed to get role")
	}

	botMember, err := bot.GetMember(guild.ID, common.BotUser.ID)
	if err != nil {
		return errors.WithMessage(err, "failed to get bot member")
	}

	if botMember == nil {
		return errors.New("failed to get bot member")
	}

	botPerms := dstate.CalculateBasePermissions(guild.ID, guild.OwnerID, guild.Roles, botMember.User.ID, botMember.Member.Roles)
	botPerms &= discordgo.PermissionManageRoles
	if botPerms == 0 {
		return errors.New("bot cannot manage the target role (missing permissions)")
	}
	botHighestRole := bot.MemberHighestRole(guild, botMember)
	if common.IsRoleAbove(targetRole, botHighestRole) {
		return errors.New("bot cannot manage the target role (role hierarchy)")
	}

	return nil
}

// Check if any bulk role operation is active (including autorole)
func isAnyBulkRoleOperationActive(guildID int64) bool {
	if IsBulkRoleOperationActive(guildID) {
		return true
	}
	var autoroleStatus int
	common.RedisPool.Do(radix.Cmd(&autoroleStatus, "GET", "autorole:"+discordgo.StrID(guildID)+":fullscan_status"))
	return autoroleStatus > 0
}

// Start bulk role operation
func (config *BulkRoleConfig) startBulkRoleOperation() error {
	guildID := config.GuildID
	if isAnyBulkRoleOperationActive(guildID) {
		return errors.New("A bulk role operation is already in progress (including autorole retroactive scan)")
	}
	if isRateLimitActive(guildID) {
		remaining := getRemainingCooldown(guildID)
		return errors.Errorf("Rate limit active. Please wait %d seconds before starting another operation", remaining)
	}

	if config.TargetRole == 0 {
		return errors.New("Target role is required")
	}

	if err := config.canBotAssignRole(); err != nil {
		return errors.WithMessage(err, "insufficient permissions")
	}

	err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "7200", strconv.Itoa(BulkRoleStarted)))
	if err != nil {
		return errors.WithMessage(err, "Failed to set initial status")
	}
	config.setBulkRoleCooldown()
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleProcessed(guildID), "0"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleResults(guildID), "0"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleChunksProcessed(guildID), "0"))

	session := bot.ShardManager.SessionForGuild(guildID)
	query := ""
	nonce := strconv.Itoa(int(guildID))
	session.GatewayManager.RequestGuildMembersComplex(&discordgo.RequestGuildMembersData{
		GuildID: guildID,
		Nonce:   nonce,
		Limit:   0,
		Query:   &query,
	})
	logger.WithField("guild", guildID).Info("Bulk role operation started")

	return nil
}

// forceOperationCompletion handles stuck operations by forcing them to complete
func (config *BulkRoleConfig) forceOperationCompletion(errorMsg string) {
	guildID := config.GuildID

	var setnx int
	common.RedisPool.Do(radix.Cmd(&setnx, "SETNX", RedisKeyBulkRoleFinalized(guildID), "1"))
	if setnx == 0 {
		return
	}
	common.RedisPool.Do(radix.Cmd(nil, "EXPIRE", RedisKeyBulkRoleFinalized(guildID), "600"))

	config, err := GetBulkRoleConfig(guildID)
	var processed int
	if err == nil {
		common.RedisPool.Do(radix.Cmd(&processed, "GET", RedisKeyBulkRoleProcessed(guildID)))
	}

	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleStatus(guildID), strconv.Itoa(BulkRoleCompleted)))
	common.RedisPool.Do(radix.Cmd(nil, "DEL",
		RedisKeyBulkRoleStatus(guildID),
		RedisKeyBulkRoleProcessed(guildID),
		RedisKeyBulkRoleResults(guildID)))
	if err == nil {
		config.sendNotificationAlert("Failed", processed, 0, errorMsg)
	}

	logger.WithField("guild", guildID).Info("Bulk role operation force-completed due to timeout/stuck state")
}

// Cancel bulk role operation
func (config *BulkRoleConfig) cancelBulkRoleOperation() error {
	guildID := config.GuildID
	if !IsBulkRoleOperationActive(guildID) {
		return nil
	}

	// Finalize guard
	var setnx int
	common.RedisPool.Do(radix.Cmd(&setnx, "SETNX", RedisKeyBulkRoleFinalized(guildID), "1"))
	if setnx == 0 {
		return nil
	}
	common.RedisPool.Do(radix.Cmd(nil, "EXPIRE", RedisKeyBulkRoleFinalized(guildID), "600"))

	// Set status to cancelled first so running chunks can detect it
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleStatus(guildID), strconv.Itoa(BulkRoleCancelled)))

	// Give running chunks a moment to detect the cancellation
	time.Sleep(time.Millisecond * 100)

	config, err := GetBulkRoleConfig(guildID)
	if err == nil {
		var processed int
		common.RedisPool.Do(radix.Cmd(&processed, "GET", RedisKeyBulkRoleProcessed(guildID)))
		config.sendNotificationAlert("Cancelled", processed, 0, "")
	}

	// Clean up all Redis keys
	common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyBulkRoleStatus(guildID), RedisKeyBulkRoleMembers(guildID), RedisKeyBulkRoleProcessed(guildID), RedisKeyBulkRoleResults(guildID), RedisKeyBulkRoleChunksProcessed(guildID)))

	logger.WithField("guild", guildID).Info("Bulk role operation cancelled")
	return nil
}

// Get bulk role operation status
func (config *BulkRoleConfig) getBulkRoleStatus() (int, int, int, error) {
	var status, processed, results int

	guildID := config.GuildID
	err := common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))
	if err != nil {
		return 0, 0, 0, err
	}

	common.RedisPool.Do(radix.Cmd(&processed, "GET", RedisKeyBulkRoleProcessed(guildID)))
	common.RedisPool.Do(radix.Cmd(&results, "GET", RedisKeyBulkRoleResults(guildID)))

	return status, processed, results, nil
}

func (config *BulkRoleConfig) setBulkRoleCooldown() {
	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleCooldown(config.GuildID), "30", "1"))
}

func (config *BulkRoleConfig) filterString() string {
	const maxFieldLength = 1000 // Leave some buffer below Discord's 1024 char limit
	var prefix string
	switch config.FilterType {
	case "has_roles":
		prefix = "Has roles"
	case "missing_roles":
		prefix = "Missing roles"
	case "all":
		prefix = "All members"
	case "bots":
		prefix = "Bots"
	case "humans":
		prefix = "Humans"
	case "joined_after":
		prefix = "Joined after"
	case "joined_before":
		prefix = "Joined before"
	default:
		prefix = "Roles"
	}

	if len(config.FilterRoleIDs) == 0 {
		return prefix
	}

	// Build the suffix first
	var suffix string
	if config.FilterRequireAll {
		suffix = " (must have ALL)"
	} else {
		suffix = " (must have ANY)"
	}

	// Start building the role list
	roleText := prefix + ": "
	availableLength := maxFieldLength - len(roleText) - len(suffix)

	var addedRoles []string
	totalLength := 0

	for i, roleID := range config.FilterRoleIDs {
		roleStr := fmt.Sprintf("<@&%d>", roleID)
		separator := ""
		if i > 0 {
			separator = ", "
		}

		testLength := totalLength + len(separator) + len(roleStr)

		// Check if adding this role would exceed the limit
		if testLength > availableLength {
			// We need to truncate
			remaining := len(config.FilterRoleIDs) - len(addedRoles)
			truncationMsg := fmt.Sprintf(", ... and %d more", remaining)

			// Make sure we have room for the truncation message
			if totalLength+len(truncationMsg) <= availableLength {
				roleText += strings.Join(addedRoles, ", ") + truncationMsg + suffix
			} else {
				// Not enough room even for truncation message, show fewer roles
				for j := len(addedRoles) - 1; j >= 0; j-- {
					testText := strings.Join(addedRoles[:j+1], ", ") + truncationMsg
					if len(testText) <= availableLength {
						roleText += testText + suffix
						break
					}
				}
				// If still too long, just show count
				if len(roleText) <= len(prefix)+2 {
					return fmt.Sprintf("%s: %d roles selected%s", prefix, len(config.FilterRoleIDs), suffix)
				}
			}
			return roleText
		}

		addedRoles = append(addedRoles, roleStr)
		totalLength = testLength
	}

	roleText += strings.Join(addedRoles, ", ") + suffix
	return roleText
}

func (config *BulkRoleConfig) sendNotificationAlert(status string, processedCount int, resultsCount int, errorMsg string) {
	if config.NotificationChannel == 0 {
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:     "Bulk Role Operation " + status,
		Color:     0x00ff00,
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Bulk Role",
		},
	}
	switch status {
	case "Completed":
		embed.Color = 0x00ff00
		embed.Title = "✅ Bulk Role Operation Completed"
	case "Failed":
		embed.Color = 0xff0000
		embed.Title = "❌ Bulk Role Operation Failed"
	case "Cancelled":
		embed.Color = 0xffa500
		embed.Title = "⏹️ Bulk Role Operation Cancelled"
	}
	filterString := config.filterString()

	// Final safety check: if filter details are still too long, use a simple fallback
	if len(filterString) > 1024 {
		switch config.FilterType {
		case "has_roles":
			filterString = fmt.Sprintf("Has %d specific roles (too many to display)%s",
				len(config.FilterRoleIDs),
				map[bool]string{true: " (must have ALL)", false: " (must have ANY)"}[config.FilterRequireAll])
		case "missing_roles":
			filterString = fmt.Sprintf("Missing %d specific roles (too many to display)%s",
				len(config.FilterRoleIDs),
				map[bool]string{true: " (must be missing ALL)", false: " (must be missing ANY)"}[config.FilterRequireAll])
		default:
			filterString = config.FilterType
		}
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "Operation",
			Value:  strings.Title(config.Operation),
			Inline: true,
		},
		{
			Name:   "Target Role",
			Value:  fmt.Sprintf("<@&%d>", config.TargetRole),
			Inline: true,
		},
		{
			Name:   "Started By",
			Value:  fmt.Sprintf("<@%d>", config.StartedBy),
			Inline: true,
		},
		{
			Name:   "Filter Criteria",
			Value:  filterString,
			Inline: false,
		},
		{
			Name:   "Members Processed",
			Value:  strconv.Itoa(processedCount),
			Inline: true,
		},
		{
			Name:   "Changes Made",
			Value:  strconv.Itoa(resultsCount),
			Inline: true,
		},
	}

	if status == "Failed" && errorMsg != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Error",
			Value: errorMsg,
		})
	}

	messageContent := fmt.Sprintf("Alert for bulk role operation started by <@%d>", config.StartedBy)
	msg := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		AllowedMentions: discordgo.AllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		},
		Content: messageContent,
	}
	_, err := bot.ShardManager.SessionForGuild(config.GuildID).ChannelMessageSendComplex(config.NotificationChannel, msg)
	if err != nil {
		logger.WithError(err).WithField("guild", config.GuildID).WithField("channel", config.NotificationChannel).Error("Failed to send notification alert")
	}
}
