package moderation

//go:generate esc -o assets_gen.go -pkg moderation -ignore ".go" assets/

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/docs"
	"github.com/jonas747/yagpdb/logs"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ActionMuted    = "Muted"
	ActionUnMuted  = "Unmuted"
	ActionKicked   = "Kicked"
	ActionBanned   = "Banned"
	ActionUnbanned = "Unbanned"
	ActionWarned   = "Warned"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Moderation"
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

	scheduledevents.RegisterEventHandler("unmute", handleUnMute)
	scheduledevents.RegisterEventHandler("mod_unban", handleUnban)
	configstore.RegisterConfig(configstore.SQL, &Config{})
	common.GORM.AutoMigrate(&Config{}, &WarningModel{}, &MuteModel{})

	docs.AddPage("Moderation", FSMustString(false, "/assets/help-page.md"), nil)
}

func handleUnMute(data string) error {

	split := strings.Split(data, ":")
	if len(split) < 2 {
		logrus.Error("Invalid unmute event", data)
		return nil // Can't re-schedule an invalid event..
	}

	guildID, _ := strconv.ParseInt(split[0], 10, 64)
	userID, _ := strconv.ParseInt(split[1], 10, 64)

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
			return nil // Discord api ok, something else went wrong. do not reschedule
		}

		return err
	}

	rc := common.MustGetRedisClient()
	defer common.RedisPool.Put(rc)

	err = MuteUnmuteUser(nil, rc, false, guildID, 0, bot.State.User(true).User, "Mute Duration Expired", member, 0)
	if err != ErrNoMuteRole {

		if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
			return nil // Discord api ok, something else went wrong. do not reschedule
		}

		return err
	}
	return nil
}

func handleUnban(data string) error {

	split := strings.Split(data, ":")
	if len(split) < 2 {
		logrus.Error("Invalid unban event", data)
		return nil // Can't re-schedule an invalid event..
	}

	guildID, _ := strconv.ParseInt(split[0], 10, 64)
	userID, _ := strconv.ParseInt(split[1], 10, 64)

	g := bot.State.Guild(true, guildID)
	if g == nil {
		logrus.WithField("guild", guildID).Error("Unban scheduled for guild now in state")
		return nil
	}

	common.RedisPool.Cmd("SETEX", RedisKeyUnbannedUser(guildID, userID), 30, 1)

	err := common.BotSession.GuildBanDelete(guildID, userID)
	if err != nil {
		logrus.WithField("guild", guildID).WithError(err).Error("Failed unbanning user")
	}

	return nil
}

func GetConfig(guildID int64) (*Config, error) {
	var config Config
	err := configstore.Cached.GetGuildConfig(context.Background(), guildID, &config)
	if err == configstore.ErrNotFound {
		err = nil
	}
	return &config, err
}

type Punishment int

const (
	PunishmentKick Punishment = iota
	PunishmentBan
)

func CreateModlogEmbed(channelID int64, author *discordgo.User, action string, target *discordgo.User, reason, logLink string) error {
	emptyAuthor := false
	if author == nil {
		emptyAuthor = true
		author = &discordgo.User{
			ID:            0,
			Username:      "Unknown",
			Discriminator: "????",
		}
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s#%s (ID %d)", author.Username, author.Discriminator, author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		},
		Description: fmt.Sprintf("**%s %s**#%s *(ID %d)*\n📄**Reason:** %s", action, target.Username, target.Discriminator, target.ID, reason),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: discordgo.EndpointUserAvatar(target.ID, target.Avatar),
		},
	}

	if strings.HasPrefix(action, ActionMuted) {
		embed.Color = 0x57728e
		embed.Description = "🔇" + embed.Description
	} else if strings.HasPrefix(action, ActionUnMuted) || action == ActionUnbanned {
		embed.Description = "🔊" + embed.Description
		embed.Color = 0x62c65f
	} else if strings.HasPrefix(action, ActionBanned) {
		embed.Description = "🔨" + embed.Description
		embed.Color = 0xd64848
	} else if strings.HasPrefix(action, ActionKicked) {
		embed.Description = "👢" + embed.Description
		embed.Color = 0xf2a013
	} else {
		embed.Description = "⚠" + embed.Description
		embed.Color = 0xfca253
	}

	if logLink != "" {
		embed.Description += " ([Logs](" + logLink + "))"
	}

	m, err := common.BotSession.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return err
	}

	if emptyAuthor {
		placeholder := fmt.Sprintf("Asssign an author and reason to this using **'reason %d your-reason-here`**", m.ID)
		updateEmbedReason(nil, placeholder, embed)
		_, err = common.BotSession.ChannelMessageEditEmbed(channelID, m.ID, embed)
	}
	return err
}

var (
	logsRegex = regexp.MustCompile(`\(\[Logs\]\(.*\)\)`)
)

func updateEmbedReason(author *discordgo.User, reason string, embed *discordgo.MessageEmbed) {
	const checkStr = "📄**Reason:**"
	index := strings.Index(embed.Description, checkStr)
	withoutReason := embed.Description[:index+len(checkStr)]

	logsLink := logsRegex.FindString(embed.Description)
	if logsLink != "" {
		logsLink = " " + logsLink
	}

	embed.Description = withoutReason + " " + reason + logsLink

	if author != nil {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s#%s (ID %d)", author.Username, author.Discriminator, author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		}
	}
}

func getConfig(guildID int64, config *Config) (*Config, error) {
	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

// Kick or bans someone, uploading a hasebin log, and sending the report tmessage in the action channel
func punish(config *Config, p Punishment, guildID, channelID int64, author *discordgo.User, reason string, user *discordgo.User, duration time.Duration) error {
	if author == nil {
		author = &discordgo.User{
			ID:            0,
			Username:      "Unknown",
			Discriminator: "????",
		}
	}

	config, err := getConfig(guildID, config)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	actionStr := ""
	if p == PunishmentKick {
		actionStr = "Kicked"
	} else {
		actionStr = "Banned"
		if duration > 0 {
			actionStr += " (" + common.HumanizeDuration(common.DurationPrecisionMinutes, duration) + ")"
		}
	}

	actionChannel, _ := strconv.ParseInt(config.ActionChannel, 10, 64)
	if actionChannel == 0 {
		actionChannel = channelID
	}

	dmMsg := ""
	if p == PunishmentKick {
		dmMsg = config.KickMessage
	} else {
		dmMsg = config.BanMessage
	}

	if dmMsg == "" {
		dmMsg = "You were " + actionStr + "\nReason: {{.Reason}}"
	}

	gs := bot.State.Guild(true, guildID)

	member, err := bot.GetMember(guildID, user.ID)
	if err != nil {
		logrus.WithError(err).WithField("guild", gs.ID()).Info("Failed retrieving member")
		member = &discordgo.Member{User: user}
	}

	ctx := templates.NewContext(bot.State.User(true).User, gs, nil, member)
	ctx.Data["Reason"] = reason
	ctx.Data["Duration"] = duration
	ctx.Data["HumanDuration"] = common.HumanizeDuration(common.DurationPrecisionMinutes, duration)
	ctx.Data["Author"] = author
	ctx.SentDM = true
	executed, err := ctx.Execute(nil, dmMsg)
	if err != nil {
		logrus.WithError(err).WithField("guild", gs.ID()).Warn("Failed executing pusnishment DM")
		executed = "Failed executing template."
	}

	gs.RLock()
	gName := "**" + gs.Guild.Name + ":** "
	gs.RUnlock()

	err = bot.SendDM(user.ID, gName+executed)
	if err != nil {
		logrus.WithError(err).Warn("Failed sending punishment DM")
	}

	logLink := ""
	if channelID != 0 {
		logLink = CreateLogs(guildID, channelID, author)
	}

	switch p {
	case PunishmentKick:
		err = common.BotSession.GuildMemberDeleteWithReason(guildID, user.ID, author.Username+"#"+author.Discriminator+": "+reason)
	case PunishmentBan:
		err = common.BotSession.GuildBanCreateWithReason(guildID, user.ID, author.Username+"#"+author.Discriminator+": "+reason, 1)
	}

	if err != nil {
		return err
	}

	logrus.Println("MODERATION:", author.Username, actionStr, user.Username, "cause", reason)

	err = CreateModlogEmbed(actionChannel, author, actionStr, user, reason, logLink)
	return err
}

func KickUser(config *Config, guildID, channelID int64, author *discordgo.User, reason string, user *discordgo.User) error {
	config, err := getConfig(guildID, config)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	err = punish(config, PunishmentKick, guildID, channelID, author, reason, user, 0)
	if err != nil {
		return err
	}

	if !config.DeleteMessagesOnKick {
		return nil
	}

	_, err = DeleteMessages(channelID, user.ID, 100, 100)
	return err
}

func DeleteMessages(channelID int64, filterUser int64, deleteNum, fetchNum int) (int, error) {
	msgs, err := bot.GetMessages(channelID, fetchNum, false)
	if err != nil {
		return 0, err
	}

	toDelete := make([]int64, 0)
	now := time.Now()
	for i := len(msgs) - 1; i >= 0; i-- {
		if filterUser == 0 || msgs[i].Author.ID == filterUser {

			parsedCreatedAt, _ := msgs[i].Timestamp.Parse()
			// Can only bulk delete messages up to 2 weeks (but add 1 minute buffer account for time sync issues and other smallies)
			if now.Sub(parsedCreatedAt) > (time.Hour*24*14)-time.Minute {
				continue
			}

			toDelete = append(toDelete, msgs[i].ID)
			//log.Println("Deleting", msgs[i].ContentWithMentionsReplaced())
			if len(toDelete) >= deleteNum || len(toDelete) >= 100 {
				break
			}
		}
	}

	if len(toDelete) < 1 {
		return 0, nil
	}

	if len(toDelete) < 1 {
		return 0, nil
	} else if len(toDelete) == 1 {
		err = common.BotSession.ChannelMessageDelete(channelID, toDelete[0])
	} else {
		err = common.BotSession.ChannelMessagesBulkDelete(channelID, toDelete)
	}

	return len(toDelete), err
}

func BanUserWithDuration(client *redis.Client, config *Config, guildID, channelID int64, author *discordgo.User, reason string, user *discordgo.User, duration time.Duration) error {
	// Set a key in redis that marks that this user has appeared in the modlog already
	client.Cmd("SETEX", RedisKeyBannedUser(guildID, user.ID), 60, 1)
	err := punish(config, PunishmentBan, guildID, channelID, author, reason, user, duration)
	if err != nil {
		return err
	}

	if duration > 0 {
		err = scheduledevents.ScheduleEvent(client, "mod_unban", discordgo.StrID(guildID)+":"+discordgo.StrID(user.ID), time.Now().Add(duration))
		if err != nil {
			return errors.WithMessage(err, "pusnish,sched_unban")
		}
	}

	return nil
}

func BanUser(client *redis.Client, config *Config, guildID, channelID int64, author *discordgo.User, reason string, user *discordgo.User) error {
	return BanUserWithDuration(client, config, guildID, channelID, author, reason, user, 0)
}

var (
	ErrNoMuteRole = errors.New("No mute role")
)

// Unmut or mute a user, ignore duration if unmuting
func MuteUnmuteUser(config *Config, client *redis.Client, mute bool, guildID, channelID int64, author *discordgo.User, reason string, member *discordgo.Member, duration int) error {
	config, err := getConfig(guildID, config)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	if config.MuteRole == "" {
		return ErrNoMuteRole
	}

	// To avoid unexpected things from happening, make sure were only updating the mute of the player 1 place at a time
	lockKey := RedisKeyLockedMute(guildID, member.User.ID)
	err = common.BlockingLockRedisKey(client, lockKey, time.Second*15, 15)
	if err != nil {
		return common.ErrWithCaller(err)
	}
	defer common.UnlockRedisKey(client, lockKey)

	logChannel, _ := strconv.ParseInt(config.ActionChannel, 10, 64)
	if config.ActionChannel == "" {
		logChannel = channelID
	}

	user := member.User

	// Look for existing mute
	currentMute := MuteModel{}
	err = common.GORM.Where(&MuteModel{UserID: member.User.ID, GuildID: guildID}).First(&currentMute).Error
	alreadyMuted := err != gorm.ErrRecordNotFound
	if err != nil && err != gorm.ErrRecordNotFound {
		return common.ErrWithCaller(err)
	}

	// Insert/update the mute entry in the database
	if !alreadyMuted {
		currentMute = MuteModel{
			UserID:  member.User.ID,
			GuildID: guildID,
		}
	}

	if author != nil {
		currentMute.AuthorID = author.ID
	}

	currentMute.Reason = reason
	currentMute.ExpiresAt = time.Now().Add(time.Minute * time.Duration(duration))

	if mute {
		// Apply the roles to the user
		removedRoles, err := AddMemberMuteRole(config, member)
		if err != nil {
			return errors.WithMessage(err, "AddMemberMuteRole")
		}

		currentMute.RemovedRoles = removedRoles
		err = common.GORM.Save(&currentMute).Error
		if err != nil {
			return errors.WithMessage(err, "failed inserting/updating mute")
		}

		err = scheduledevents.ScheduleEvent(client, "unmute", discordgo.StrID(guildID)+":"+discordgo.StrID(user.ID), time.Now().Add(time.Minute*time.Duration(duration)))
		client.Cmd("SETEX", RedisKeyMutedUser(guildID, user.ID), duration*60, 1)
		if err != nil {
			return errors.WithMessage(err, "failed scheduling unmute")
		}
	} else {
		// Remove the mute role, and give back the role the bot took
		err = RemoveMemberMuteRole(config, member, currentMute)
		if err != nil {
			return errors.WithMessage(err, "failed removing mute role")
		}

		if alreadyMuted {
			common.GORM.Delete(&currentMute)
			client.Cmd("DEL", RedisKeyMutedUser(guildID, user.ID))
		}

		err = scheduledevents.RemoveEvent(client, "unmute", discordgo.StrID(guildID)+":"+discordgo.StrID(user.ID))
		if err != nil {
			logrus.WithError(err).Error("Failed scheduling/removing unmute event")
		}
	}

	// Upload logs
	logLink := ""
	if channelID != 0 && mute {
		logLink = CreateLogs(guildID, channelID, author)
	}

	action := ""
	if mute {
		action = fmt.Sprintf("Muted (%d min)", duration)
	} else {
		action = "Unmuted"
	}

	dmMsg := "You have been " + action
	if reason != "" {
		dmMsg += "\n**Reason:** " + reason
	}
	bot.SendDM(user.ID, dmMsg)

	if logChannel != 0 {
		return CreateModlogEmbed(logChannel, author, action, user, reason, logLink)
	}

	return nil
}

func AddMemberMuteRole(config *Config, member *discordgo.Member) (removedRoles []int64, err error) {
	removedRoles = make([]int64, 0, len(config.MuteRemoveRoles))
	newMemberRoles := make([]string, 0, len(member.Roles))
	newMemberRoles = append(newMemberRoles, config.MuteRole)

	for _, r := range member.Roles {
		if config.IntMuteRole() == r {
			continue
		}

		if common.ContainsInt64Slice(config.MuteRemoveRoles, r) {
			removedRoles = append(removedRoles, r)
		} else {
			newMemberRoles = append(newMemberRoles, strconv.FormatInt(r, 10))
		}
	}

	err = common.BotSession.GuildMemberEdit(config.GuildID, member.User.ID, newMemberRoles)
	return
}

func RemoveMemberMuteRole(config *Config, member *discordgo.Member, mute MuteModel) (err error) {

	newMemberRoles := make([]string, 0, len(member.Roles)+len(config.MuteRemoveRoles))

	for _, v := range member.Roles {
		if v != config.IntMuteRole() {
			newMemberRoles = append(newMemberRoles, strconv.FormatInt(v, 10))
		}
	}

	for _, v := range mute.RemovedRoles {
		if !common.ContainsInt64Slice(member.Roles, v) {
			newMemberRoles = append(newMemberRoles, strconv.FormatInt(v, 10))
		}
	}

	err = common.BotSession.GuildMemberEdit(config.GuildID, member.User.ID, newMemberRoles)

	return
}

func WarnUser(config *Config, guildID, channelID int64, author *discordgo.User, target *discordgo.User, message string) error {
	warning := &WarningModel{
		GuildID:               guildID,
		UserID:                discordgo.StrID(target.ID),
		AuthorID:              discordgo.StrID(author.ID),
		AuthorUsernameDiscrim: author.Username + "#" + author.Discriminator,

		Message: message,
	}

	config, err := getConfig(guildID, config)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	if config.WarnIncludeChannelLogs {
		warning.LogsLink = CreateLogs(guildID, channelID, author)
	}

	err = common.GORM.Create(warning).Error
	if err != nil {
		return common.ErrWithCaller(err)
	}

	gs := bot.State.Guild(true, guildID)
	gs.RLock()
	name := gs.Guild.Name
	gs.RUnlock()

	bot.SendDM(target.ID, fmt.Sprintf("**%s**: You have been warned for: %s", name, message))

	if config.WarnSendToModlog && config.ActionChannel != "" {
		parsedActionChannel, _ := strconv.ParseInt(config.ActionChannel, 10, 64)
		err = CreateModlogEmbed(parsedActionChannel, author, ActionWarned, target, message, warning.LogsLink)
		if err != nil {
			return common.ErrWithCaller(err)
		}
	}

	return nil
}

func CreateLogs(guildID, channelID int64, user *discordgo.User) string {
	lgs, err := logs.CreateChannelLog(nil, guildID, channelID, user.Username, user.ID, 100)
	if err != nil {
		if err == logs.ErrChannelBlacklisted {
			return ""
		}
		logrus.WithError(err).Error("Log Creation Failed")
		return "Log Creation Failed"
	}
	return lgs.Link()
}
