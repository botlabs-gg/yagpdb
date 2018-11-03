package moderation

import (
	"errors"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
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
var _ bot.ShardMigrationHandler = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(ModerationCommands...)
}

func (p *Plugin) BotInit() {
	scheduledevents.RegisterEventHandler("unmute", handleUnMute)
	scheduledevents.RegisterEventHandler("mod_unban", handleUnban)

	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleGuildBanAddRemove), eventsystem.EventGuildBanAdd, eventsystem.EventGuildBanRemove)
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleGuildMemberRemove), eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandler(LockMemberMuteMW(HandleMemberJoin), eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandler(LockMemberMuteMW(HandleGuildMemberUpdate), eventsystem.EventGuildMemberUpdate)

	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(HandleChannelCreateUpdate, eventsystem.EventChannelUpdate, eventsystem.EventChannelUpdate)

	pubsub.AddHandler("mod_refresh_mute_override", HandleRefreshMuteOverrides, nil)
}

func (p *Plugin) GuildMigrated(gs *dstate.GuildState, toThisSlave bool) {
	if !toThisSlave {
		return
	}

	go RefreshMuteOverrides(gs.ID)
}

func HandleRefreshMuteOverrides(evt *pubsub.Event) {
	RefreshMuteOverrides(evt.TargetGuildInt)
}

func HandleGuildCreate(evt *eventsystem.EventData) {
	gc := evt.GuildCreate()
	RefreshMuteOverrides(gc.ID)
}

// Refreshes the mute override on the channel, currently it only adds it.
func RefreshMuteOverrides(guildID int64) {

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

	if guild.Role(true, config.IntMuteRole()) == nil {
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

func HandleChannelCreateUpdate(evt *eventsystem.EventData) {
	var channel *discordgo.Channel
	if evt.Type == eventsystem.EventChannelCreate {
		channel = evt.ChannelCreate().Channel
	} else {
		channel = evt.ChannelUpdate().Channel
	}

	if channel.GuildID == 0 {
		return
	}

	config, err := GetConfig(channel.GuildID)
	if err != nil {
		return
	}

	if config.MuteRole == "" || !config.MuteManageRole {
		return
	}

	RefreshMuteOverrideForChannel(config, channel)
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

	allows := 0
	denies := MuteDeniedChannelPerms
	changed := true

	if override != nil {
		allows = override.Allow
		denies = override.Deny
		changed = false

		if (allows & MuteDeniedChannelPerms) != 0 {
			// One of the mute permissions was in the allows, remove it
			allows &= ^MuteDeniedChannelPerms
			changed = true
		}

		if (denies & MuteDeniedChannelPerms) != MuteDeniedChannelPerms {
			// Missing one of the mute permissions
			denies |= MuteDeniedChannelPerms
			changed = true
		}
	}

	if changed {
		common.BotSession.ChannelPermissionSet(channel.ID, config.IntMuteRole(), "role", allows, denies)
	}
}

func HandleGuildBanAddRemove(evt *eventsystem.EventData) {
	var user *discordgo.User
	var guildID int64
	var action ModlogAction

	botPerformed := false

	switch evt.Type {
	case eventsystem.EventGuildBanAdd:

		guildID = evt.GuildBanAdd().GuildID
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
		guildID = evt.GuildBanRemove().GuildID

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
		logrus.WithError(err).WithField("guild", guildID).Error("Failed retrieving config")
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

	err = CreateModlogEmbed(config.IntActionChannel(), author, action, user, reason, "")
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("Failed sending " + action.Prefix + " log message")
	}
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) {
	data := evt.GuildMemberRemove()

	config, err := GetConfig(data.GuildID)
	if err != nil {
		logrus.WithError(err).WithField("guild", data.GuildID).Error("Failed retrieving config")
		return
	}

	if config.IntActionChannel() == 0 {
		return
	}

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

	err = CreateModlogEmbed(config.IntActionChannel(), author, MAKick, data.User, entry.Reason, "")
	if err != nil {
		logrus.WithError(err).WithField("guild", data.GuildID).Error("Failed sending kick log message")
	}
}

// Since updating mutes are now a complex operation with removing roles and whatnot,
// to avoid weird bugs from happening we lock it so it can only be updated one place per user
func LockMemberMuteMW(next func(evt *eventsystem.EventData)) func(evt *eventsystem.EventData) {
	return func(evt *eventsystem.EventData) {
		var userID int64
		var guild int64
		// TODO: add utility functions to the eventdata struct for fetching things like these?
		if evt.Type == eventsystem.EventGuildMemberAdd {
			userID = evt.GuildMemberAdd().User.ID
			guild = evt.GuildMemberAdd().GuildID
		} else if evt.Type == eventsystem.EventGuildMemberUpdate {
			userID = evt.GuildMemberUpdate().User.ID
			guild = evt.GuildMemberUpdate().GuildID
		} else {
			panic("Unknown event in lock memebr mute middleware")
		}

		// If there's less than 2 seconds of the mute left, don't bother doing anything
		var muteLeft int
		common.RedisPool.Do(radix.Cmd(&muteLeft, "TTL", RedisKeyMutedUser(guild, userID)))
		if muteLeft < 5 {
			return
		}

		LockMute(userID)
		defer UnlockMute(userID)

		// The situation may have changed at this point, check again
		common.RedisPool.Do(radix.Cmd(&muteLeft, "TTL", RedisKeyMutedUser(guild, userID)))
		if muteLeft < 5 {
			return
		}

		next(evt)
	}
}

func HandleMemberJoin(evt *eventsystem.EventData) {
	c := evt.GuildMemberAdd()

	config, err := GetConfig(c.GuildID)
	if err != nil {
		logrus.WithError(err).WithField("guild", c.GuildID).Error("Failed retrieving config")
		return
	}
	if config.MuteRole == "" {
		return
	}

	logrus.WithField("guild", c.GuildID).WithField("user", c.User.ID).Info("Assigning back mute role after member rejoined")
	err = common.BotSession.GuildMemberRoleAdd(c.GuildID, c.User.ID, config.IntMuteRole())
	if err != nil {
		logrus.WithField("guild", c.GuildID).WithError(err).Error("Failed assigning mute role")
	}
}

func HandleGuildMemberUpdate(evt *eventsystem.EventData) {
	c := evt.GuildMemberUpdate()

	config, err := GetConfig(c.GuildID)
	if err != nil {
		logrus.WithError(err).WithField("guild", c.GuildID).Error("Failed retrieving config")
		return
	}
	if config.MuteRole == "" {
		return
	}

	guild := bot.State.Guild(true, c.GuildID)
	if guild == nil {
		return
	}
	role := guild.Role(true, config.IntMuteRole())
	if role == nil {
		return // Probably deleted the mute role, do nothing then
	}

	logrus.WithField("guild", c.Member.GuildID).WithField("user", c.User.ID).Info("Giving back mute roles arr")

	removedRoles, err := AddMemberMuteRole(config, c.Member.User.ID, c.Member.Roles)
	if err != nil {
		logrus.WithError(err).Error("Failed adding mute role to user in member update")
	}

	if len(removedRoles) < 1 {
		return
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		logrus.WithError(err).Error("Failed starting transaction")
		return
	}

	// Append the removed roles to the removed_roles array column, if they don't already exist in it
	const queryStr = "UPDATE muted_users SET removed_roles = array_append(removed_roles, $3 ) WHERE user_id=$2 AND guild_id=$1 AND NOT ($3 = ANY(removed_roles));"
	for _, v := range removedRoles {
		_, err := tx.Exec(queryStr, c.GuildID, c.Member.User.ID, v)
		if err != nil {
			logrus.WithError(err).Error("Failed updating removed roles")
			break
		}
	}

	err = tx.Commit()
	if err != nil {
		logrus.WithError(err).Error("Failed comitting transaction")
	}
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
