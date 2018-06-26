package moderation

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"strconv"
	"strings"
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
}

func getConfigIfNotSet(guildID int64, config *Config) (*Config, error) {
	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

func GetConfig(guildID int64) (*Config, error) {
	var config Config
	err := configstore.Cached.GetGuildConfig(context.Background(), guildID, &config)
	if err == configstore.ErrNotFound {
		err = nil
	}
	return &config, err
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
	if errors.Cause(err) != ErrNoMuteRole {

		if cast, ok := errors.Cause(err).(*discordgo.RESTError); ok && cast.Message != nil {
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
