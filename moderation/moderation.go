package moderation

//go:generate esc -o assets_gen.go -pkg moderation -ignore ".go" assets/

import (
	"fmt"
	"github.com/Sirupsen/logrus"
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

func RedisKeyMutedUser(guildID, userID string) string {
	return "moderation_muted_user:" + guildID + ":" + userID
}

func RedisKeyBannedUser(guildID, userID string) string {
	return "moderation_banned_user:" + guildID + ":" + userID
}

func RedisKeyUnbannedUser(guildID, userID string) string {
	return "moderation_unbanned_user:" + guildID + ":" + userID
}

func RegisterPlugin() {
	plugin := &Plugin{}

	common.RegisterPlugin(plugin)

	scheduledevents.RegisterEventHandler("unmute", handleUnMute)
	scheduledevents.RegisterEventHandler("mod_unban", handleUnban)
	configstore.RegisterConfig(configstore.SQL, &Config{})
	common.GORM.AutoMigrate(&Config{}, &WarningModel{})

	docs.AddPage("Moderation", FSMustString(false, "/assets/help-page.md"), nil)
}

func handleUnMute(data string) error {

	split := strings.Split(data, ":")
	if len(split) < 2 {
		logrus.Error("Invalid unmute event", data)
		return nil // Can't re-schedule an invalid event..
	}

	guildID := split[0]
	userID := split[1]

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
			return nil // Discord api ok, something else went wrong. do not reschedule
		}

		return err
	}

	err = MuteUnmuteUser(nil, nil, false, guildID, "", bot.State.User(true).User, "Mute Duration Expired", member, 0)
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

	guildID := split[0]
	userID := split[1]

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

type Config struct {
	configstore.GuildConfigModel

	// Kick command
	KickEnabled          bool
	DeleteMessagesOnKick bool
	KickReasonOptional   bool
	KickMessage          string `valid:"template,1900"`

	// Ban
	BanEnabled        bool
	BanReasonOptional bool
	BanMessage        string `valid:"template,1900"`

	// Mute/unmute
	MuteEnabled          bool
	MuteRole             string `valid:"role,true"`
	MuteReasonOptional   bool
	UnmuteReasonOptional bool

	// Warn
	WarnCommandsEnabled    bool
	WarnIncludeChannelLogs bool
	WarnSendToModlog       bool

	// Misc
	CleanEnabled  bool
	ReportEnabled bool
	ActionChannel string `valid:"channel,true"`
	ReportChannel string `valid:"channel,true"`
	LogUnbans     bool
	LogBans       bool
}

func (c *Config) GetName() string {
	return "moderation"
}

func (c *Config) TableName() string {
	return "moderation_configs"
}

func (c *Config) Save(client *redis.Client, guildID string) error {
	parsedId, err := strconv.ParseInt(guildID, 10, 64)
	if err != nil {
		return err
	}
	c.GuildID = parsedId
	return configstore.SQL.SetGuildConfig(context.Background(), c)
}

func GetConfig(guildID string) (*Config, error) {
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

func CreateModlogEmbed(channelID string, author *discordgo.User, action string, target *discordgo.User, reason, logLink string) error {
	emptyAuthor := false
	if author == nil {
		emptyAuthor = true
		author = &discordgo.User{
			ID:            "??",
			Username:      "Unknown",
			Discriminator: "????",
		}
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s#%s (ID %s)", author.Username, author.Discriminator, author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		},
		Description: fmt.Sprintf("**%s %s**#%s *(ID %s)*\n📄**Reason:** %s", action, target.Username, target.Discriminator, target.ID, reason),
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
		placeholder := fmt.Sprintf("Asssign an author and reason to this using **'reason %s your-reason-here`**", m.ID)
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
			Name:    fmt.Sprintf("%s#%s (ID %s)", author.Username, author.Discriminator, author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		}
	}

}

func getConfig(guildID string, config *Config) (*Config, error) {
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
func punish(config *Config, p Punishment, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User, duration time.Duration) error {
	if author == nil {
		author = &discordgo.User{
			ID:            "??",
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

	actionChannel := config.ActionChannel
	if actionChannel == "" {
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
		logrus.WithError(err).WithField("guild", gs.ID()).Warn("Failed executing pusnishment dm")
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
	if channelID != "" {
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

func KickUser(config *Config, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User) error {
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

func DeleteMessages(channelID string, filterUser string, deleteNum, fetchNum int) (int, error) {
	msgs, err := bot.GetMessages(channelID, fetchNum, false)
	if err != nil {
		return 0, err
	}

	toDelete := make([]string, 0)
	now := time.Now()
	for i := len(msgs) - 1; i >= 0; i-- {
		if filterUser == "" || msgs[i].Author.ID == filterUser {

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

func BanUserWithDuration(client *redis.Client, config *Config, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User, duration time.Duration) error {
	// Set a key in redis that marks that this user has appeared in the modlog already
	client.Cmd("SETEX", RedisKeyBannedUser(guildID, user.ID), 60, 1)
	err := punish(config, PunishmentBan, guildID, channelID, author, reason, user, duration)
	if err != nil {
		return err
	}

	if duration > 0 {
		err = scheduledevents.ScheduleEvent(client, "mod_unban", guildID+":"+user.ID, time.Now().Add(duration))
		if err != nil {
			return errors.WithMessage(err, "pusnish,sched_unban")
		}
	}

	return nil
}

func BanUser(client *redis.Client, config *Config, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User) error {
	return BanUserWithDuration(client, config, guildID, channelID, author, reason, user, 0)
}

var (
	ErrNoMuteRole = errors.New("No mute role")
)

// Unmut or mute a user, ignore duration if unmuting
func MuteUnmuteUser(config *Config, client *redis.Client, mute bool, guildID, channelID string, author *discordgo.User, reason string, member *discordgo.Member, duration int) error {
	config, err := getConfig(guildID, config)
	if err != nil {
		return common.ErrWithCaller(err)
	}

	if config.MuteRole == "" {
		return ErrNoMuteRole
	}

	logChannel := config.ActionChannel
	if config.ActionChannel == "" {
		logChannel = channelID
	}

	user := member.User

	isMuted := false
	for _, v := range member.Roles {
		if v == config.MuteRole {
			isMuted = true
			break
		}
	}

	// Mute or unmute if needed
	if mute && !isMuted {
		// Mute
		err = common.BotSession.GuildMemberRoleAdd(guildID, user.ID, config.MuteRole)
		logrus.Info("Added mute role yoooo")
	} else if !mute && isMuted {
		// Unmute
		err = common.BotSession.GuildMemberRoleRemove(guildID, user.ID, config.MuteRole)
	} else if !mute && !isMuted {
		// Trying to unmute an unmuted user? e.e
		return nil
	}

	if err != nil {
		return err
	}

	// Either remove the scheduled unmute or schedule an unmute in the future
	if mute {
		err = scheduledevents.ScheduleEvent(client, "unmute", guildID+":"+user.ID, time.Now().Add(time.Minute*time.Duration(duration)))
		client.Cmd("SETEX", RedisKeyMutedUser(guildID, user.ID), duration*60, 1)
	} else {
		if client != nil {
			err = scheduledevents.RemoveEvent(client, "unmute", guildID+":"+user.ID)
		}
	}
	if err != nil {
		logrus.WithError(err).Error("Failed scheduling/removing unmute event")
	}

	// Upload logs
	logLink := ""
	if channelID != "" && mute {
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

	if logChannel != "" {
		return CreateModlogEmbed(logChannel, author, action, user, reason, logLink)
	}

	return nil
}

type WarningModel struct {
	common.SmallModel
	GuildID  int64 `gorm:"index"`
	UserID   string
	AuthorID string

	// Username and discrim for author incase he/she leaves
	AuthorUsernameDiscrim string

	Message  string
	LogsLink string
}

func (w *WarningModel) TableName() string {
	return "moderation_warnings"
}

func WarnUser(config *Config, guildID, channelID string, author *discordgo.User, target *discordgo.User, message string) error {
	warning := &WarningModel{
		GuildID:               common.MustParseInt(guildID),
		UserID:                target.ID,
		AuthorID:              author.ID,
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
		err = CreateModlogEmbed(config.ActionChannel, author, ActionWarned, target, message, warning.LogsLink)
		if err != nil {
			return common.ErrWithCaller(err)
		}
	}

	return nil
}

func CreateLogs(guildID, channelID string, user *discordgo.User) string {
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
