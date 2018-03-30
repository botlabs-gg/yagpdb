package bot

import (
	"context"
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

var (
	Cache = cache.New(time.Minute, time.Minute)
)

func ContextSession(ctx context.Context) *discordgo.Session {
	return ctx.Value(common.ContextKeyDiscordSession).(*discordgo.Session)
}

func ContextRedis(ctx context.Context) *redis.Client {
	return ctx.Value(common.ContextKeyRedis).(*redis.Client)
}

func RedisWrapper(inner eventsystem.Handler) eventsystem.Handler {
	return func(evt *eventsystem.EventData) {
		r, err := common.RedisPool.Get()
		if err != nil {
			logrus.WithError(err).WithField("evt", evt.Type.String()).Error("Failed retrieving redis client")
			return
		}

		defer func() {
			common.RedisPool.Put(r)
		}()

		inner(evt.WithContext(context.WithValue(evt.Context(), common.ContextKeyRedis, r)))
	}
}

func GetCreatePrivateChannel(user int64) (*discordgo.Channel, error) {

	State.RLock()
	defer State.RUnlock()
	for _, c := range State.PrivateChannels {
		if c.Recipient() != nil && c.Recipient().ID == user {
			return c.Copy(true, false), nil
		}
	}

	channel, err := common.BotSession.UserChannelCreate(user)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func SendDM(user int64, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}

	channel, err := GetCreatePrivateChannel(user)
	if err != nil {
		return err
	}

	_, err = common.BotSession.ChannelMessageSend(channel.ID, msg)
	return err
}

var (
	ErrStartingUp    = errors.New("Starting up, caches are being filled...")
	ErrGuildNotFound = errors.New("Guild not found")
)

func AdminOrPerm(needed int, userID, channelID int64) (bool, error) {
	channel := State.Channel(true, channelID)
	if channel == nil {
		return false, errors.New("Channel not found")
	}

	// Ensure the member is in state
	GetMember(channel.Guild.ID(), userID)
	perms, err := channel.Guild.MemberPermissions(true, channelID, userID)
	if err != nil {
		return false, err
	}

	if err != nil {
		return false, err
	}

	if perms&needed != 0 {
		return true, nil
	}

	if perms&discordgo.PermissionManageServer != 0 || perms&discordgo.PermissionAdministrator != 0 {
		return true, nil
	}

	return false, nil
}
