package bulkrole

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
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

// Check if any bulk role operation is active (including autorole)
func isAnyBulkRoleOperationActive(guildID int64) bool {
	if IsBulkRoleOperationActive(guildID) {
		return true
	}
	var autoroleStatus int
	common.RedisPool.Do(radix.Cmd(&autoroleStatus, "GET", "autorole:"+discordgo.StrID(guildID)+":fullscan_status"))
	return autoroleStatus > 0
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

	if !isAnyBulkRoleOperationActive(guildID) {
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

	if chunk.ChunkCount == 0 {
		logger.WithField("guild", guildID).Warn("Received chunk with invalid ChunkCount, forcing completion")
		sendNotificationAlert(guildID, config, "Failed", 0, 0, "Invalid chunk data received from Discord")

		forceOperationCompletion(guildID)
		return
	}

	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterating)))
	go processBulkRoleChunk(guildID, config, chunk)
}

// Process a chunk of members for bulk role operations
func processBulkRoleChunk(guildID int64, config *BulkRoleConfig, chunk *discordgo.GuildMembersChunk) {
	// Local per-chunk counters to avoid races across concurrent chunk goroutines
	chunkProcessed := 0
	chunkResults := 0

	lastTimeStatusRefreshed := time.Now()

	for _, member := range chunk.Members {
		if isBulkRoleCancelled(guildID) {
			return
		}

		// Count this member as processed regardless of whether a change is needed
		chunkProcessed++

		if !memberMeetsFilter(member, config) {
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

		if chunk.ChunkCount <= 0 {
			logger.WithField("guild", guildID).WithField("chunkIndex", chunk.ChunkIndex).WithField("chunkCount", chunk.ChunkCount).Warn("Invalid chunk completion data, forcing completion")
			sendNotificationAlert(guildID, config, "Failed", 0, 0, "Invalid chunk completion data received from Discord")

			forceOperationCompletion(guildID)
			return
		}
		err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterationDone)))
		if err != nil {
			logger.WithError(err).Error("Failed marking bulk role iteration complete")
		}
		common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleStatus(guildID), strconv.Itoa(BulkRoleCompleted)))
		setBulkRoleCooldown(guildID)
		var finalProcessed, finalResults int
		common.RedisPool.Do(radix.Cmd(&finalProcessed, "GET", RedisKeyBulkRoleProcessed(guildID)))
		common.RedisPool.Do(radix.Cmd(&finalResults, "GET", RedisKeyBulkRoleResults(guildID)))
		sendNotificationAlert(guildID, config, "Completed", finalProcessed, finalResults, "")
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
func memberMeetsFilter(member *discordgo.Member, config *BulkRoleConfig) bool {
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

// Start bulk role operation
func startBulkRoleOperation(guildID int64, config *BulkRoleConfig) error {
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
	err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "7200", strconv.Itoa(BulkRoleStarted)))
	if err != nil {
		return errors.WithMessage(err, "Failed to set initial status")
	}
	setBulkRoleCooldown(guildID)
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleProcessed(guildID), "0"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleResults(guildID), "0"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleChunksProcessed(guildID), "0"))
	go startFallbackCompletionTimer(guildID)
	session := bot.ShardManager.SessionForGuild(guildID)
	query := ""
	nonce := strconv.Itoa(int(guildID))
	session.GatewayManager.RequestGuildMembersComplex(&discordgo.RequestGuildMembersData{
		GuildID: guildID,
		Nonce:   nonce,
		Limit:   0,
		Query:   &query,
	})

	return nil
}

// startFallbackCompletionTimer provides a safety net for completion detection
// This handles edge cases where chunk counting might fail
func startFallbackCompletionTimer(guildID int64) {
	time.Sleep(time.Second * 60) // 1 minute

	var status int
	common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))

	if status == BulkRoleStarted || status == BulkRoleIterating {
		go startFallbackCompletionTimer(guildID)
		return
	}

	if status == BulkRoleCompleted || status == BulkRoleCancelled {
		return
	}

	if status == BulkRoleIterating {
		go startAggressiveCompletionTimer(guildID)
	}
}

// startAggressiveCompletionTimer handles cases where chunks might be delayed or missing
func startAggressiveCompletionTimer(guildID int64) {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	timeout := time.After(time.Minute * 10)

	for {
		select {
		case <-ticker.C:
			var status int
			common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))

			if status == BulkRoleCompleted || status == BulkRoleCancelled {
				return
			}

			if status == BulkRoleIterating {
				continue
			}

			// If we're stuck in an unexpected state, force completion
			if status == BulkRoleStarted || status == BulkRoleIterationDone {
				logger.WithField("guild", guildID).Warn("Bulk role operation stuck, forcing completion")
				forceOperationCompletion(guildID)
				return
			}

		case <-timeout:
			logger.WithField("guild", guildID).Warn("Bulk role operation timeout reached, forcing completion")
			forceOperationCompletion(guildID)
			return
		}
	}
}

// forceOperationCompletion handles stuck operations by forcing them to complete
func forceOperationCompletion(guildID int64) {
	// Finalize guard
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
		sendNotificationAlert(guildID, config, "Failed", processed, 0, "Operation was force-completed due to timeout or stuck state")
	}

	logger.WithField("guild", guildID).Info("Bulk role operation force-completed due to timeout/stuck state")
}

// Cancel bulk role operation
func cancelBulkRoleOperation(guildID int64) error {
	if !IsBulkRoleOperationActive(guildID) {
		return errors.New("No bulk role operation in progress")
	}

	// Finalize guard
	var setnx int
	common.RedisPool.Do(radix.Cmd(&setnx, "SETNX", RedisKeyBulkRoleFinalized(guildID), "1"))
	if setnx == 0 {
		return nil
	}
	common.RedisPool.Do(radix.Cmd(nil, "EXPIRE", RedisKeyBulkRoleFinalized(guildID), "600"))

	config, err := GetBulkRoleConfig(guildID)
	if err == nil {
		var processed int
		common.RedisPool.Do(radix.Cmd(&processed, "GET", RedisKeyBulkRoleProcessed(guildID)))
		sendNotificationAlert(guildID, config, "Cancelled", processed, 0, "")
	}
	common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyBulkRoleStatus(guildID), RedisKeyBulkRoleMembers(guildID), RedisKeyBulkRoleProcessed(guildID), RedisKeyBulkRoleResults(guildID)))

	return nil
}

// Get bulk role operation status
func getBulkRoleStatus(guildID int64) (int, int, int, error) {
	var status, processed, results int

	err := common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))
	if err != nil {
		return 0, 0, 0, err
	}

	common.RedisPool.Do(radix.Cmd(&processed, "GET", RedisKeyBulkRoleProcessed(guildID)))
	common.RedisPool.Do(radix.Cmd(&results, "GET", RedisKeyBulkRoleResults(guildID)))

	return status, processed, results, nil
}

func setBulkRoleCooldown(guildID int64) {
	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleCooldown(guildID), "30", "1"))
}

// Check if bulkrole operation is active (for autorole to use)
func IsBulkRoleOperationActive(guildID int64) bool {
	var status int
	common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))
	return status > 0
}

// formatRoleList formats a list of role IDs with smart truncation to stay within Discord limits
func formatRoleList(roleIDs []int64, prefix string, requireAll bool) string {
	const maxFieldLength = 1000 // Leave some buffer below Discord's 1024 char limit

	if len(roleIDs) == 0 {
		return prefix + " (none selected)"
	}

	// Build the suffix first
	var suffix string
	if requireAll {
		suffix = " (must have ALL)"
	} else {
		suffix = " (must have ANY)"
	}

	// Start building the role list
	roleText := prefix + ": "
	availableLength := maxFieldLength - len(roleText) - len(suffix)

	var addedRoles []string
	totalLength := 0

	for i, roleID := range roleIDs {
		roleStr := fmt.Sprintf("<@&%d>", roleID)
		separator := ""
		if i > 0 {
			separator = ", "
		}

		testLength := totalLength + len(separator) + len(roleStr)

		// Check if adding this role would exceed the limit
		if testLength > availableLength {
			// We need to truncate
			remaining := len(roleIDs) - len(addedRoles)
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
					return fmt.Sprintf("%s: %d roles selected%s", prefix, len(roleIDs), suffix)
				}
			}
			return roleText
		}

		addedRoles = append(addedRoles, roleStr)
		totalLength = testLength
	}

	// All roles fit
	roleText += strings.Join(addedRoles, ", ") + suffix
	return roleText
}

// formatFilterDetails returns a formatted string with filter-specific information
func formatFilterDetails(guildID int64, config *BulkRoleConfig) string {
	switch config.FilterType {
	case "all":
		return "All members"
	case "bots":
		return "Bot accounts only"
	case "humans":
		return "Human accounts only"
	case "has_roles":
		return formatRoleList(config.FilterRoleIDs, "Has roles", config.FilterRequireAll)
	case "missing_roles":
		return formatRoleList(config.FilterRoleIDs, "Missing roles", config.FilterRequireAll)
	case "joined_after":
		if config.FilterDate != "" {
			return fmt.Sprintf("Joined after %s", config.FilterDate)
		}
		return "Joined after (no date specified)"
	case "joined_before":
		if config.FilterDate != "" {
			return fmt.Sprintf("Joined before %s", config.FilterDate)
		}
		return "Joined before (no date specified)"
	default:
		return config.FilterType
	}
}

// sendNotificationAlert sends a notification to the configured channel about operation status
func sendNotificationAlert(guildID int64, config *BulkRoleConfig, status string, processedCount int, resultsCount int, errorMsg string) {
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
	filterDetails := formatFilterDetails(guildID, config)

	// Final safety check: if filter details are still too long, use a simple fallback
	if len(filterDetails) > 1024 {
		switch config.FilterType {
		case "has_roles":
			filterDetails = fmt.Sprintf("Has %d specific roles (too many to display)%s",
				len(config.FilterRoleIDs),
				map[bool]string{true: " (must have ALL)", false: " (must have ANY)"}[config.FilterRequireAll])
		case "missing_roles":
			filterDetails = fmt.Sprintf("Missing %d specific roles (too many to display)%s",
				len(config.FilterRoleIDs),
				map[bool]string{true: " (must be missing ALL)", false: " (must be missing ANY)"}[config.FilterRequireAll])
		default:
			filterDetails = config.FilterType
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
			Value:  filterDetails,
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
	_, err := bot.ShardManager.SessionForGuild(guildID).ChannelMessageSendComplex(config.NotificationChannel, msg)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("channel", config.NotificationChannel).Error("Failed to send notification alert")
	}
}
