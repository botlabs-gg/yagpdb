package moderation

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dshardorchestrator/v2"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	seventsmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/mediocregopher/radix/v3"
)

var (
	ErrFailedPerms = errors.New("Failed retrieving perms")
)

type ContextKey int

const (
	ContextKeyConfig ContextKey = iota
)

const MuteDeniedChannelPerms = discordgo.PermissionSendMessages | discordgo.PermissionVoiceSpeak

var _ commands.CommandProvider = (*Plugin)(nil)
var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.ShardMigrationReceiver = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, ModerationCommands...)
}

func (p *Plugin) BotInit() {
	// scheduledevents.RegisterEventHandler("unmute", handleUnMuteLegacy)
	// scheduledevents.RegisterEventHandler("mod_unban", handleUnbanLegacy)
	scheduledevents2.RegisterHandler("moderation_unmute", ScheduledUnmuteData{}, handleScheduledUnmute)
	scheduledevents2.RegisterHandler("moderation_unban", ScheduledUnbanData{}, handleScheduledUnban)
	scheduledevents2.RegisterLegacyMigrater("unmute", handleMigrateScheduledUnmute)
	scheduledevents2.RegisterLegacyMigrater("mod_unban", handleMigrateScheduledUnban)

	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleGuildBanAddRemove), eventsystem.EventGuildBanAdd, eventsystem.EventGuildBanRemove)
	eventsystem.AddHandlerAsyncLast(p, HandleGuildMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandlerAsyncLast(p, LockMemberMuteMW(HandleMemberJoin), eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandlerAsyncLast(p, LockMemberMuteMW(HandleGuildMemberUpdate), eventsystem.EventGuildMemberUpdate)

	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLast(p, HandleChannelCreateUpdate, eventsystem.EventChannelCreate, eventsystem.EventChannelUpdate)

	pubsub.AddHandler("mod_refresh_mute_override", HandleRefreshMuteOverrides, nil)
}

type ScheduledUnmuteData struct {
	UserID int64 `json:"user_id"`
}

type ScheduledUnbanData struct {
	UserID int64 `json:"user_id"`
}

func (p *Plugin) ShardMigrationReceive(evt dshardorchestrator.EventType, data interface{}) {
	if evt == bot.EvtGuildState {
		gs := data.(*dstate.GuildState)
		go RefreshMuteOverrides(gs.ID)
	}
}

func HandleRefreshMuteOverrides(evt *pubsub.Event) {
	RefreshMuteOverrides(evt.TargetGuildInt)
}

var started = time.Now()

func HandleGuildCreate(evt *eventsystem.EventData) {
	if !evt.HasFeatureFlag(featureFlagMuteRoleManaged) {
		return // nothing to do
	}

	gc := evt.GuildCreate()

	// relieve startup preasure, sleep for up to 10 minutes
	if time.Since(started) < time.Minute {
		sleep := time.Second * time.Duration(100+rand.Intn(600))
		time.Sleep(sleep)
	}

	RefreshMuteOverrides(gc.ID)
}

// Refreshes the mute override on the channel, currently it only adds it.
func RefreshMuteOverrides(guildID int64) {
	if !featureflags.GuildHasFlagOrLogError(guildID, featureFlagMuteRoleManaged) {
		return // nothing to do
	}

	config, err := GetConfig(guildID)
	if err != nil {
		return
	}

	if config.MuteRole == "" || !config.MuteManageRole {
		return
	}

	guild := bot.State.Guild(true, guildID)
	if guild == nil {
		return // Still starting up and haven't received the guild yet
	}

	if guild.RoleCopy(true, config.IntMuteRole()) == nil {
		return
	}

	guild.RLock()
	channelsCopy := make([]*discordgo.Channel, 0, len(guild.Channels))
	for _, v := range guild.Channels {
		channelsCopy = append(channelsCopy, v.DGoCopy())
	}
	guild.RUnlock()

	for _, v := range channelsCopy {
		RefreshMuteOverrideForChannel(config, v)
	}
}

func HandleChannelCreateUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	var channel *discordgo.Channel
	if evt.Type == eventsystem.EventChannelCreate {
		channel = evt.ChannelCreate().Channel
	} else {
		channel = evt.ChannelUpdate().Channel
	}

	if channel.GuildID == 0 {
		return false, nil
	}

	config, err := GetConfig(channel.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if config.MuteRole == "" || !config.MuteManageRole {
		return false, nil
	}

	RefreshMuteOverrideForChannel(config, channel)

	return false, nil
}

func RefreshMuteOverrideForChannel(config *Config, channel *discordgo.Channel) {
	// Ignore the channel
	if common.ContainsInt64Slice(config.MuteIgnoreChannels, channel.ID) {
		return
	}

	if !bot.BotProbablyHasPermission(channel.GuildID, channel.ID, discordgo.PermissionManageRoles) {
		return
	}

	var override *discordgo.PermissionOverwrite

	// Check for existing override
	for _, v := range channel.PermissionOverwrites {
		if v.Type == "role" && v.ID == config.IntMuteRole() {
			override = v
			break
		}
	}

	MuteDeniedChannelPermsFinal := MuteDeniedChannelPerms
	if config.MuteDisallowReactionAdd {
		MuteDeniedChannelPermsFinal = MuteDeniedChannelPermsFinal | discordgo.PermissionAddReactions
	}
	allows := 0
	denies := MuteDeniedChannelPermsFinal
	changed := true

	if override != nil {
		allows = override.Allow
		denies = override.Deny
		changed = false

		if (allows & MuteDeniedChannelPermsFinal) != 0 {
			// One of the mute permissions was in the allows, remove it
			allows &= ^MuteDeniedChannelPermsFinal
			changed = true
		}

		if (denies & MuteDeniedChannelPermsFinal) != MuteDeniedChannelPermsFinal {
			// Missing one of the mute permissions
			denies |= MuteDeniedChannelPermsFinal
			changed = true
		}
	}

	if changed {
		common.BotSession.ChannelPermissionSet(channel.ID, config.IntMuteRole(), "role", allows, denies)
	}
}

func HandleGuildBanAddRemove(evt *eventsystem.EventData) {
	var user *discordgo.User
	var guildID = evt.GS.ID
	var action ModlogAction

	botPerformed := false

	switch evt.Type {
	case eventsystem.EventGuildBanAdd:

		user = evt.GuildBanAdd().User
		action = MABanned

		var i int
		common.RedisPool.Do(radix.Cmd(&i, "GET", RedisKeyBannedUser(guildID, user.ID)))
		if i > 0 {
			// The bot banned the user earlier, don't make duplicate entries in the modlog
			common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyBannedUser(guildID, user.ID)))
			return
		}

	case eventsystem.EventGuildBanRemove:

		action = MAUnbanned
		user = evt.GuildBanRemove().User

		var i int
		common.RedisPool.Do(radix.Cmd(&i, "GET", RedisKeyUnbannedUser(guildID, user.ID)))
		if i > 0 {
			// The bot was the one that performed the unban
			common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyUnbannedUser(guildID, user.ID)))
			botPerformed = true
		}

	default:
		return
	}

	config, err := GetConfig(guildID)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("Failed retrieving config")
		return
	}

	if config.IntActionChannel() == 0 {
		return
	}

	var author *discordgo.User
	reason := ""

	if !botPerformed {
		// If we poll it too fast then there sometimes wont be a audit log entry
		time.Sleep(time.Second * 3)

		auditlogAction := discordgo.AuditLogActionMemberBanAdd
		if evt.Type == eventsystem.EventGuildBanRemove {
			auditlogAction = discordgo.AuditLogActionMemberBanRemove
		}

		var entry *discordgo.AuditLogEntry
		author, entry = FindAuditLogEntry(guildID, auditlogAction, user.ID, -1)
		if entry != nil {
			reason = entry.Reason
		}
	}

	if (action == MAUnbanned && !config.LogUnbans && !botPerformed) ||
		(action == MABanned && !config.LogBans) {
		return
	}

	// The bot only unbans people in the case of timed bans
	if botPerformed {
		author = common.BotUser
		reason = "Timed ban expired"
	}

	err = CreateModlogEmbed(config, author, action, user, reason, "")
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("Failed sending " + action.Prefix + " log message")
	}
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) (retry bool, err error) {
	data := evt.GuildMemberRemove()

	config, err := GetConfig(data.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if config.IntActionChannel() == 0 {
		return false, nil
	}

	go checkAuditLogMemberRemoved(config, data)
	return false, nil
}

func checkAuditLogMemberRemoved(config *Config, data *discordgo.GuildMemberRemove) {
	// If we poll the audit log too fast then there sometimes wont be a audit log entry
	time.Sleep(time.Second * 3)

	author, entry := FindAuditLogEntry(data.GuildID, discordgo.AuditLogActionMemberKick, data.User.ID, time.Second*5)
	if entry == nil || author == nil {
		return
	}

	if author.ID == common.BotUser.ID {
		// Bot performed the kick, don't make duplicate modlog entries
		return
	}

	err := CreateModlogEmbed(config, author, MAKick, data.User, entry.Reason, "")
	if err != nil {
		logger.WithError(err).WithField("guild", data.GuildID).Error("Failed sending kick log message")
	}
}

// Since updating mutes are now a complex operation with removing roles and whatnot,
// to avoid weird bugs from happening we lock it so it can only be updated one place per user
func LockMemberMuteMW(next eventsystem.HandlerFunc) eventsystem.HandlerFunc {
	return func(evt *eventsystem.EventData) (retry bool, err error) {
		var userID int64
		// TODO: add utility functions to the eventdata struct for fetching things like these?
		if evt.Type == eventsystem.EventGuildMemberAdd {
			userID = evt.GuildMemberAdd().User.ID
		} else if evt.Type == eventsystem.EventGuildMemberUpdate {
			userID = evt.GuildMemberUpdate().User.ID
		} else {
			panic("Unknown event in lock memebr mute middleware")
		}

		LockMute(userID)
		defer UnlockMute(userID)

		guildID := evt.GS.ID

		var currentMute MuteModel
		err = common.GORM.Where(MuteModel{UserID: userID, GuildID: guildID}).First(&currentMute).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}

			return false, errors.WithStackIf(err)
		}

		// Don't bother doing anythign if this mute is almost up
		if !currentMute.ExpiresAt.IsZero() && currentMute.ExpiresAt.Sub(time.Now()) < 5*time.Second {
			return false, nil
		}

		return next(evt)
	}
}

func HandleMemberJoin(evt *eventsystem.EventData) (retry bool, err error) {
	c := evt.GuildMemberAdd()

	config, err := GetConfig(c.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if config.MuteRole == "" {
		return false, nil
	}

	err = common.BotSession.GuildMemberRoleAdd(c.GuildID, c.User.ID, config.IntMuteRole())
	if err != nil {
		return bot.CheckDiscordErrRetry(err), errors.WithStackIf(err)
	}

	return false, nil
}

func HandleGuildMemberUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	c := evt.GuildMemberUpdate()

	config, err := GetConfig(c.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}
	if config.MuteRole == "" {
		return false, nil
	}

	guild := evt.GS

	role := guild.RoleCopy(true, config.IntMuteRole())
	if role == nil {
		return false, nil // Probably deleted the mute role, do nothing then
	}

	removedRoles, err := AddMemberMuteRole(config, c.Member.User.ID, c.Member.Roles)
	if err != nil {
		return bot.CheckDiscordErrRetry(err), errors.WithStackIf(err)
	}

	if len(removedRoles) < 1 {
		return false, nil
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	// Append the removed roles to the removed_roles array column, if they don't already exist in it
	const queryStr = "UPDATE muted_users SET removed_roles = array_append(removed_roles, $3 ) WHERE user_id=$2 AND guild_id=$1 AND NOT ($3 = ANY(removed_roles));"
	for _, v := range removedRoles {
		_, err := tx.Exec(queryStr, c.GuildID, c.Member.User.ID, v)
		if err != nil {
			tx.Rollback()
			return true, errors.WithStackIf(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	return false, nil
}

func FindAuditLogEntry(guildID int64, typ int, targetUser int64, within time.Duration) (author *discordgo.User, entry *discordgo.AuditLogEntry) {
	auditlog, err := common.BotSession.GuildAuditLog(guildID, 0, 0, typ, 10)
	if err != nil {
		return nil, nil
	}

	for _, entry := range auditlog.AuditLogEntries {
		if entry.TargetID == targetUser {

			if within != -1 {
				t := bot.SnowflakeToTime(entry.ID)
				if time.Since(t) > within {
					return nil, nil
				}
			}

			// Find the user details from the id
			for _, v := range auditlog.Users {
				if v.ID == entry.UserID {
					return v, entry
				}
			}

			break
		}
	}

	return nil, nil
}

func handleMigrateScheduledUnmute(t time.Time, data string) error {
	split := strings.Split(data, ":")
	if len(split) < 2 {
		logger.Error("invalid unmute event", data)
		return nil
	}

	guildID, _ := strconv.ParseInt(split[0], 10, 64)
	userID, _ := strconv.ParseInt(split[1], 10, 64)

	return scheduledevents2.ScheduleEvent("moderation_unmute", guildID, t, &ScheduledUnmuteData{
		UserID: userID,
	})
}

func handleMigrateScheduledUnban(t time.Time, data string) error {
	split := strings.Split(data, ":")
	if len(split) < 2 {
		logger.Error("Invalid unban event", data)
		return nil // Can't re-schedule an invalid event..
	}

	guildID, _ := strconv.ParseInt(split[0], 10, 64)
	userID, _ := strconv.ParseInt(split[1], 10, 64)

	return scheduledevents2.ScheduleEvent("moderation_unban", guildID, t, &ScheduledUnbanData{
		UserID: userID,
	})
}

func handleScheduledUnmute(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {
	unmuteData := data.(*ScheduledUnmuteData)

	member, err := bot.GetMember(evt.GuildID, unmuteData.UserID)
	if err != nil {
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	err = MuteUnmuteUser(nil, false, evt.GuildID, nil, nil, common.BotUser, "Mute Duration Expired", member, 0)
	if errors.Cause(err) != ErrNoMuteRole {
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	return false, nil
}

func handleScheduledUnban(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {
	unbanData := data.(*ScheduledUnbanData)

	guildID := evt.GuildID
	userID := unbanData.UserID

	g := bot.State.Guild(true, guildID)
	if g == nil {
		logger.WithField("guild", guildID).Error("Unban scheduled for guild not in state")
		return false, nil
	}

	common.RedisPool.Do(radix.FlatCmd(nil, "SETEX", RedisKeyUnbannedUser(guildID, userID), 30, 1))

	err = common.BotSession.GuildBanDelete(guildID, userID)
	if err != nil {
		logger.WithField("guild", guildID).WithError(err).Error("failed unbanning user")
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	return false, nil
}
