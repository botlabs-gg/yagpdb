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
	"strings"
	"time"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Moderation"
}

func (p *Plugin) MigrateSQL(client *redis.Client, guildID string, guildIDInt int64) error {
	config, err := GetConfig(guildID)
	if err != nil {
		return err
	}

	config.GuildID = guildIDInt
	err = common.SQL.Save(config).Error
	return err
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
		return err
	}

	err = MuteUnmuteUser(nil, nil, false, guildID, "", "YAGPDB", "Mute Duration expired", member, 0)
	if err != ErrNoMuteRole {

		if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
			if cast.Message.Code == 10007 {
				return nil // MemberNot found
			}
		}

		return err
	}
	return nil
}

type Config struct {
	configstore.GuildConfigModel
	BanEnabled           bool   `schema:"ban_enabled"`
	KickEnabled          bool   `schema:"kick_enabled"`
	CleanEnabled         bool   `schema:"clean_enabled"`
	ReportEnabled        bool   `schema:"report_enabled"`
	MuteEnabled          bool   `schema:"mute_enabled"`
	DeleteMessagesOnKick bool   `schema:"kick_delete_messages"`
	ActionChannel        string `schema:"action_channel" valid:"channel,true"`
	ReportChannel        string `schema:"report_channel" valid:"channel,true"`
	BanMessage           string `schema:"ban_message" valid:"template,1900"`
	KickMessage          string `schema:"kick_message" valid:"template,1900"`
	MuteRole             string `schema:"mute_role" valid:"role,true"`
}

func (c *Config) GetName() string {
	return "moderation"
}

func (c *Config) TableName() string {
	return "moderation_configs"
}

func (c *Config) Save(client *redis.Client, guildID string) error {
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

func GetConfigDeprecated(client *redis.Client, guildID string) (config *Config, err error) {
	client.Append("GET", "moderation_ban_enabled:"+guildID)
	client.Append("GET", "moderation_kick_enabled:"+guildID)
	client.Append("GET", "moderation_clean_enabled:"+guildID)
	client.Append("GET", "moderation_report_enabled:"+guildID)
	client.Append("GET", "moderation_action_channel:"+guildID)
	client.Append("GET", "moderation_report_channel:"+guildID)
	client.Append("GET", "moderation_ban_message:"+guildID)
	client.Append("GET", "moderation_kick_message:"+guildID)
	client.Append("GET", "moderation_kick_delete_messages:"+guildID)
	client.Append("GET", "moderation_mute_enabled:"+guildID)
	client.Append("GET", "moderation_mute_role:"+guildID)

	replies, err := common.GetRedisReplies(client, 11)
	if err != nil {
		return nil, err
	}

	// We already checked errors above, altthough if someone were to fuck shit up manually
	// Then yeah, these would be default values with errors thrown
	banEnabled, _ := replies[0].Bool()
	kickEnabled, _ := replies[1].Bool()
	cleanEnabled, _ := replies[2].Bool()
	reportEnabled, _ := replies[3].Bool()
	actionChannel, _ := replies[4].Str()
	reportChannel, _ := replies[5].Str()
	banMsg, _ := replies[6].Str()
	kickMsg, _ := replies[7].Str()
	kickDeleteMessages, _ := replies[8].Bool()
	muteEnabled, _ := replies[9].Bool()
	muteRole, _ := replies[10].Str()

	return &Config{
		BanEnabled:           banEnabled,
		KickEnabled:          kickEnabled,
		CleanEnabled:         cleanEnabled,
		ReportEnabled:        reportEnabled,
		ActionChannel:        actionChannel,
		ReportChannel:        reportChannel,
		BanMessage:           banMsg,
		KickMessage:          kickMsg,
		DeleteMessagesOnKick: kickDeleteMessages,
		MuteEnabled:          muteEnabled,
		MuteRole:             muteRole,
	}, nil
}

type Punishment int

const (
	PunishmentKick Punishment = iota
	PunishmentBan
)

func GenModlogMessage(author, action string, target *discordgo.User, reason, logLink string) string {
	msg := fmt.Sprintf("%s %s **%s**#%s *(%s)*\n**Reason:** %s", author, action, target.Username, target.Discriminator, target.ID, reason)
	if logLink != "" {
		msg += fmt.Sprintf("\n**Logs:** <%s>", logLink)
	}

	return msg
}

// Kick or bans someone, uploading a hasebin log, and sending the report tmessage in the action channel
func punish(config *Config, p Punishment, guildID, channelID, author, reason string, user *discordgo.User) error {
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

	guild := common.MustGetGuild(guildID)
	gName := "**" + guild.Name + ":** "

	err = bot.SendDM(common.BotSession, user.ID, gName+executed)
	if err != nil {
		return err
	}

	logLink := ""
	if channelID != "" {
		logs, err := logs.CreateChannelLog(channelID, author, "", 100)
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

	logrus.Println(author, actionStr, user.Username, "cause", reason)

	logMsg := GenModlogMessage(author, actionStr, user, reason, logLink)

	_, err = common.BotSession.ChannelMessageSend(actionChannel, logMsg)
	if err != nil {
		return err
	}

	return nil
}

func KickUser(config *Config, guildID, channelID, author, reason string, user *discordgo.User) error {
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

	lastMsgs, err := common.GetMessages(channelID, 100)
	if err != nil {
		return err
	}
	toDelete := make([]string, 0)

	for _, v := range lastMsgs {
		if v.Author.ID == user.ID {
			toDelete = append(toDelete, v.ID)
		}
	}

	if len(toDelete) < 1 {
		return nil
	}

	if len(toDelete) == 1 {
		common.BotSession.ChannelMessageDelete(channelID, toDelete[0])
	} else {
		common.BotSession.ChannelMessagesBulkDelete(channelID, toDelete)
	}

	return nil
}

func BanUser(config *Config, guildID, channelID, author, reason string, user *discordgo.User) error {
	return punish(config, PunishmentBan, guildID, channelID, author, reason, user)
}

var (
	ErrNoMuteRole = errors.New("No mute role")
)

// Unmut or mute a user, ignore duration if unmuting
func MuteUnmuteUser(config *Config, client *redis.Client, mute bool, guildID, channelID, author, reason string, member *discordgo.Member, duration int) error {
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
		newRoles := make([]string, len(member.Roles)+1)
		copy(newRoles, member.Roles)
		newRoles[len(member.Roles)] = config.MuteRole

		err = common.BotSession.GuildMemberEdit(guildID, user.ID, newRoles)
		logrus.Info("Added mute role yoooo")
	} else if !mute && isMuted {
		newRoles := make([]string, 0)
		for _, v := range member.Roles {
			if v != config.MuteRole {
				newRoles = append(newRoles, v)
			}
		}
		err = common.BotSession.GuildMemberEdit(guildID, user.ID, newRoles)
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
		logs, err := logs.CreateChannelLog(channelID, author, "", 100)
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

	msg := GenModlogMessage(author, action, user, reason, logLink)
	if logChannel != "" {
		_, err = common.BotSession.ChannelMessageSend(logChannel, msg)
		return err
	}

	return nil
}
