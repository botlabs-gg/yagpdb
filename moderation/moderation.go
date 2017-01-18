package moderation

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/logs"
	"github.com/jonas747/yagpdb/web"
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

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)
	common.RegisterScheduledEventHandler("unmute", handleUnMute)
	configstore.RegisterConfig(configstore.SQL, &Config{})
	common.SQL.AutoMigrate(&Config{})
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

	err = MuteUnmuteUser(nil, nil, false, guildID, "", bot.State.User(true).User, "Mute Duration expired", member, 0)
	if err != ErrNoMuteRole {

		if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
			return nil // Discord api ok, something else went wrong. do not reschedule
		}

		return err
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
		Description: fmt.Sprintf("**%s %s**#%s *(ID %s)*\n:notepad_spiral:**Reason:** %s", action, target.Username, target.Discriminator, target.ID, reason),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: discordgo.EndpointUserAvatar(target.ID, target.Avatar),
		},
	}

	if strings.HasPrefix(action, ActionMuted) {
		embed.Color = 0x57728e
		embed.Description = ":mute:" + embed.Description
	} else if strings.HasPrefix(action, ActionUnMuted) || action == ActionUnbanned {
		embed.Description = ":speaker:" + embed.Description
		embed.Color = 0x62c65f
	} else if strings.HasPrefix(action, ActionBanned) {
		embed.Description = ":hammer:" + embed.Description
		embed.Color = 0xd64848
	} else {
		// kick
		embed.Description = ":boot:" + embed.Description
		embed.Color = 0xf2a013
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
	const checkStr = ":notepad_spiral:**Reason:**"
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

// Kick or bans someone, uploading a hasebin log, and sending the report tmessage in the action channel
func punish(config *Config, p Punishment, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User) error {
	if author == nil {
		author = &discordgo.User{
			ID:            "??",
			Username:      "Unknown",
			Discriminator: "????",
		}
	}

	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return err
		}
	}

	actionStr := "Banned"
	if p == PunishmentKick {
		actionStr = "Kicked"
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

	executed, err := common.ParseExecuteTemplate(dmMsg, map[string]interface{}{
		"User":   user,
		"Reason": reason,
	})

	gs := bot.State.Guild(true, guildID)
	gs.RLock()
	gName := "**" + gs.Guild.Name + ":** "
	gs.RUnlock()

	err = bot.SendDM(user.ID, gName+executed)
	if err != nil {
		logrus.WithError(err).Warn("Failed sending punishment dm")
	}

	logLink := ""
	if channelID != "" {
		logs, err := logs.CreateChannelLog(channelID, author.Username, author.ID, 100)
		if err != nil {
			logLink = "Log Creation failed"
			logrus.WithError(err).Error("Log Creation failed")
		} else {
			logLink = logs.Link()
		}
	}

	switch p {
	case PunishmentKick:
		err = common.BotSession.GuildMemberDelete(guildID, user.ID)
	case PunishmentBan:

		err = common.BotSession.GuildBanCreate(guildID, user.ID, 1)
	}

	if err != nil {
		return err
	}

	logrus.Println("MODERATION:", author.Username, actionStr, user.Username, "cause", reason)

	err = CreateModlogEmbed(actionChannel, author, actionStr, user, reason, logLink)
	if err != nil {
		return err
	}

	return nil
}

func KickUser(config *Config, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User) error {
	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return err
		}
	}

	err := punish(config, PunishmentKick, guildID, channelID, author, reason, user)
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

func BanUser(client *redis.Client, config *Config, guildID, channelID string, author *discordgo.User, reason string, user *discordgo.User) error {
	client.Cmd("SETEX", RedisKeyBannedUser(guildID, user.ID), 60, 1)
	return punish(config, PunishmentBan, guildID, channelID, author, reason, user)
}

var (
	ErrNoMuteRole = errors.New("No mute role")
)

// Unmut or mute a user, ignore duration if unmuting
func MuteUnmuteUser(config *Config, client *redis.Client, mute bool, guildID, channelID string, author *discordgo.User, reason string, member *discordgo.Member, duration int) error {
	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return err
		}
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

	var err error
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
		err = common.ScheduleEvent(client, "unmute", guildID+":"+user.ID, time.Now().Add(time.Minute*time.Duration(duration)))
		client.Cmd("SETEX", RedisKeyMutedUser(guildID, user.ID), duration*60, 1)
	} else {
		if client != nil {
			err = common.RemoveScheduledEvent(client, "unmute", guildID+":"+user.ID)
		}
	}
	if err != nil {
		logrus.WithError(err).Error("Failed shceduling/removing unmute event")
	}

	// Upload logs
	logLink := ""
	if channelID != "" && mute {
		logs, err := logs.CreateChannelLog(channelID, author.Username, author.ID, 100)
		if err != nil {
			logLink = "Log Creation failed"
			logrus.WithError(err).Error("Log Creation failed")
		} else {
			logLink = logs.Link()
		}
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
