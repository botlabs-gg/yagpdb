package bulkrole

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/autorole"
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

func RedisKeyBulkRoleCancelled(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":cancelled"
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

const (
	// How long we let an operation go without hearing from discord before
	// assuming the member request was dropped.
	bulkRoleStallTimeout = time.Minute

	bulkRoleStallCheckInterval = time.Minute
	bulkRoleStallRetries       = 3
)

func RedisKeyBulkRoleLastProgress(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":last_progress"
}

func markBulkRoleProgress(guildID int64) {
	err := common.RedisPool.Do(radix.FlatCmd(nil, "SETEX", RedisKeyBulkRoleLastProgress(guildID), int(bulkRoleStallTimeout.Seconds())*4, time.Now().Unix()))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("Failed refreshing bulk role progress marker")
	}
}

func isBulkRoleStalled(guildID int64) bool {
	var lastProgress int64
	common.RedisPool.Do(radix.Cmd(&lastProgress, "GET", RedisKeyBulkRoleLastProgress(guildID)))
	if lastProgress == 0 {
		return true
	}

	return time.Since(time.Unix(lastProgress, 0)) > bulkRoleStallTimeout
}

func chunkRequestNonce(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID)
}

func getRemainingCooldown(guildID int64) int64 {
	var ttl int64
	common.RedisPool.Do(radix.Cmd(&ttl, "TTL", RedisKeyBulkRoleCooldown(guildID)))
	return ttl
}

// Handle guild member chunk for bulk role operations
func handleGuildChunk(evt *eventsystem.EventData) {
	chunk := evt.GuildMembersChunk()
	guildID := chunk.GuildID

	if !IsBulkRoleOperationActive(guildID) {
		return
	}

	if chunk.Nonce != chunkRequestNonce(guildID) {
		return
	}

	config, err := GetBulkRoleConfig(guildID)
	if err != nil {
		logger.WithError(err).Error("Failed to get bulkrole config")
		return
	}

	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterating)))
	markBulkRoleProgress(guildID)
	go config.processBulkRoleChunk(chunk)
}

// Process a chunk of members for bulk role operations
func (config *BulkRoleConfig) processBulkRoleChunk(chunk *discordgo.GuildMembersChunk) {
	defer config.markChunkProcessed(chunk)

	if err := config.canBotAssignRole(); err != nil {
		logger.WithError(err).WithField("guild", config.GuildID).Error("Bot lost permissions during bulk role operation, canceling")
		config.cancelBulkRoleOperation("Failed", "Bot lost permissions during operation")
		return
	}

	if !IsBulkRoleOperationActive(config.GuildID) {
		return
	}

	lastTimeStatusRefreshed := time.Now()

	guildID := config.GuildID
	session := bot.ShardManager.SessionForGuild(guildID)
	for _, member := range chunk.Members {
		if !IsBulkRoleOperationActive(guildID) {
			return
		}

		common.RedisPool.Do(radix.Cmd(nil, "INCR", RedisKeyBulkRoleProcessed(guildID)))

		if time.Since(lastTimeStatusRefreshed) > time.Second*50 {
			lastTimeStatusRefreshed = time.Now()
			err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "100", strconv.Itoa(BulkRoleIterating)))
			if err != nil {
				logger.WithError(err).Error("Failed refreshing bulk role iterating status")
			}
			markBulkRoleProgress(guildID)
		}

		if !config.filterMember(member) {
			continue
		}

		hasRole := slices.Contains(member.Roles, config.TargetRole)
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
			err = session.GuildMemberRoleAdd(guildID, member.User.ID, config.TargetRole)
		case "remove":
			err = session.GuildMemberRoleRemove(guildID, member.User.ID, config.TargetRole)
		}

		if err != nil {
			logger.WithError(err).WithField("guild", guildID).WithField("user", member.User.ID).Error("Failed to modify role")
			continue
		}

		common.RedisPool.Do(radix.Cmd(nil, "INCR", RedisKeyBulkRoleResults(guildID)))

		// Rate limiting
		time.Sleep(time.Millisecond * 100)
	}

}

func (config *BulkRoleConfig) markChunkProcessed(chunk *discordgo.GuildMembersChunk) {
	guildID := config.GuildID

	var doneChunks int
	common.RedisPool.Do(radix.Cmd(&doneChunks, "INCR", RedisKeyBulkRoleChunksProcessed(guildID)))

	if doneChunks >= chunk.ChunkCount {
		if !IsBulkRoleCancelled(guildID) {
			config.markBulkRoleOperationEnd("Completed", "Bulk Role operation completed")
		}
		return
	}

	logger.WithField("guild", guildID).WithField("doneChunks", doneChunks).WithField("chunkCount", chunk.ChunkCount).Debug("Processed chunk, waiting for more")
}

func hasAllRoles(member *discordgo.Member, roleIDs []int64) bool {
	for _, roleID := range roleIDs {
		if !slices.Contains(member.Roles, roleID) {
			return false
		}
	}
	return true
}

func hasAnyRole(member *discordgo.Member, roleIDs []int64) bool {
	for _, roleID := range roleIDs {
		if slices.Contains(member.Roles, roleID) {
			return true
		}
	}
	return false
}

// Check if a member meets the filter criteria.
//
// FilterRequireAll decides whether the filter's condition has to hold for every
// selected role or just one of them:
//
//	has_roles      off: has any one of them          on: has all of them
//	missing_roles  off: is missing any one of them   on: is missing all of them
func (config *BulkRoleConfig) filterMember(member *discordgo.Member) bool {
	switch config.FilterType {
	case "all":
		return true
	case "has_roles":
		if len(config.FilterRoleIDs) == 0 {
			return false
		}
		if config.FilterRequireAll {
			return hasAllRoles(member, config.FilterRoleIDs)
		}
		return hasAnyRole(member, config.FilterRoleIDs)
	case "missing_roles":
		if len(config.FilterRoleIDs) == 0 {
			return false
		}
		if config.FilterRequireAll {
			return !hasAnyRole(member, config.FilterRoleIDs)
		}
		return !hasAllRoles(member, config.FilterRoleIDs)
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
	common.RedisPool.Do(radix.Cmd(&autoroleStatus, "GET", autorole.RedisKeyFullScanStatus(guildID)))
	return autoroleStatus > 0 && autoroleStatus != autorole.FullScanCancelled
}

// Start bulk role operation
func (config *BulkRoleConfig) startBulkRoleOperation() error {
	guildID := config.GuildID
	if isAnyBulkRoleOperationActive(guildID) {
		return errors.New("A bulk role operation is already in progress (including autorole retroactive scan)")
	}
	remaining := getRemainingCooldown(guildID)
	if remaining > 0 {
		return errors.Errorf("Rate limit active. Please wait %d seconds before starting another operation", remaining)
	}

	if config.TargetRole == 0 {
		return errors.New("Target role is required")
	}

	if err := config.canBotAssignRole(); err != nil {
		return errors.WithMessage(err, "insufficient permissions")
	}

	common.RedisPool.Do(radix.Cmd(nil, "DEL",
		RedisKeyBulkRoleCancelled(guildID),
		RedisKeyBulkRoleFinalized(guildID)))

	err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleStatus(guildID), "7200", strconv.Itoa(BulkRoleStarted)))
	if err != nil {
		return errors.WithMessage(err, "Failed to set initial status")
	}
	config.setBulkRoleCooldown()
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleProcessed(guildID), "0"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleResults(guildID), "0"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleChunksProcessed(guildID), "0"))

	config.requestGuildMembers()
	markBulkRoleProgress(guildID)
	go config.watchForStall()

	logger.WithField("guild", guildID).Info("Bulk role operation started")

	return nil
}

func (config *BulkRoleConfig) requestGuildMembers() {
	guildID := config.GuildID

	session := bot.ShardManager.SessionForGuild(guildID)
	if session == nil {
		logger.WithField("guild", guildID).Error("No session for guild, cannot request members")
		return
	}

	query := ""
	session.GatewayManager.RequestGuildMembersComplex(&discordgo.RequestGuildMembersData{
		GuildID: guildID,
		Nonce:   chunkRequestNonce(guildID),
		Limit:   0,
		Query:   &query,
	})
}

// Discord silently drops a REQUEST_GUILD_MEMBERS it rate limits, which would
// otherwise leave the operation sitting at started until its status expires.
func (config *BulkRoleConfig) watchForStall() {
	guildID := config.GuildID

	ticker := time.NewTicker(bulkRoleStallCheckInterval)
	defer ticker.Stop()

	retries := 0
	for range ticker.C {
		if !IsBulkRoleOperationActive(guildID) {
			return
		}

		if !isBulkRoleStalled(guildID) {
			retries = 0
			continue
		}

		if retries >= bulkRoleStallRetries {
			logger.WithField("guild", guildID).WithField("retries", retries).Warn("No member chunks received, failing stalled bulk role operation")
			config.cancelBulkRoleOperation("Failed", "Discord never sent the member list, which usually means the request was rate limited. Please try again in a few minutes.")
			return
		}

		retries++
		logger.WithField("guild", guildID).WithField("attempt", retries).Warn("No member chunks received, re-requesting the member list")

		// a retry re-delivers every chunk, so the count so far no longer lines up
		common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleChunksProcessed(guildID), "0"))
		config.requestGuildMembers()
		markBulkRoleProgress(guildID)
	}
}

func (config *BulkRoleConfig) markBulkRoleOperationEnd(status, msg string) {
	guildID := config.GuildID

	var setnx int
	common.RedisPool.Do(radix.Cmd(&setnx, "SETNX", RedisKeyBulkRoleFinalized(guildID), "1"))
	if setnx == 0 {
		return
	}
	common.RedisPool.Do(radix.Cmd(nil, "EXPIRE", RedisKeyBulkRoleFinalized(guildID), "30"))
	_, processed, results, err := config.getBulkRoleProgress()
	if err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("Failed to get bulk role progress")
	}

	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleStatus(guildID), strconv.Itoa(BulkRoleCompleted)))
	common.RedisPool.Do(radix.Cmd(nil, "DEL",
		RedisKeyBulkRoleStatus(guildID),
		RedisKeyBulkRoleProcessed(guildID),
		RedisKeyBulkRoleResults(guildID),
		RedisKeyBulkRoleLastProgress(guildID)))

	config.sendNotificationAlert(status, processed, results, msg)
	logger.WithField("guild", guildID).Info("Bulk role operation force-completed due to timeout/stuck/cancellation state")
}

// Cancel bulk role operation
func (config *BulkRoleConfig) cancelBulkRoleOperation(reason, msg string) error {
	guildID := config.GuildID
	if !IsBulkRoleOperationActive(guildID) {
		return nil
	}

	// Set status to cancelled first so running chunks can detect it
	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleCancelled(guildID), "30", "1"))
	common.RedisPool.Do(radix.Cmd(nil, "SET", RedisKeyBulkRoleStatus(guildID), strconv.Itoa(BulkRoleCancelled)))

	// Give running chunks a moment to detect the cancellation
	time.Sleep(time.Second * 1)

	config.markBulkRoleOperationEnd(reason, msg)
	logger.WithField("guild", guildID).Info("Bulk role operation cancelled")
	return nil
}

// Get bulk role operation status
func (config *BulkRoleConfig) getBulkRoleProgress() (int, int, int, error) {
	var status, processed, results int
	err := common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(config.GuildID)))
	if err != nil {
		return 0, 0, 0, err
	}
	common.RedisPool.Do(radix.Cmd(&processed, "GET", RedisKeyBulkRoleProcessed(config.GuildID)))
	common.RedisPool.Do(radix.Cmd(&results, "GET", RedisKeyBulkRoleResults(config.GuildID)))

	return status, processed, results, nil
}

func (config *BulkRoleConfig) setBulkRoleCooldown() {
	common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyBulkRoleCooldown(config.GuildID), "30", "1"))
}

func (config *BulkRoleConfig) filterRoleString(prefix string) string {
	const maxFieldLength = 1000 // Leave some buffer below Discord's 1024 char limit
	if len(config.FilterRoleIDs) == 0 {
		return prefix
	}

	suffix := " (" + config.matchCriteriaText() + ")"

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

// matchCriteriaText describes what the ALL/ANY toggle means for the configured
// filter type, see filterMember.
func (config *BulkRoleConfig) matchCriteriaText() string {
	if config.FilterType == "missing_roles" {
		if config.FilterRequireAll {
			return "must have none of the selected roles"
		}
		return "must be missing at least one selected role"
	}

	if config.FilterRequireAll {
		return "must have all of the selected roles"
	}
	return "must have at least one of the selected roles"
}

func (config *BulkRoleConfig) filterString() string {
	var prefix string
	switch config.FilterType {
	case "has_roles":
		return config.filterRoleString("Has roles")
	case "missing_roles":
		return config.filterRoleString("Missing roles")
	case "all":
		prefix = "All members"
	case "bots":
		prefix = "Bots"
	case "humans":
		prefix = "Humans"
	case "joined_after":
		prefix = "Joined after: " + config.FilterDateParsed.Format("January 2, 2006")
	case "joined_before":
		prefix = "Joined before: " + config.FilterDateParsed.Format("January 2, 2006")
	default:
		prefix = "Roles"
	}
	return prefix
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
			filterString = fmt.Sprintf("Has %d specific roles (too many to display) (%s)",
				len(config.FilterRoleIDs), config.matchCriteriaText())
		case "missing_roles":
			filterString = fmt.Sprintf("Missing %d specific roles (too many to display) (%s)",
				len(config.FilterRoleIDs), config.matchCriteriaText())
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

	embed.Footer.Text = "Members Processed and Changes Made can be inaccurate because of discord rate limits."

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
