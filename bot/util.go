package bot

import (
	"context"
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	Cache = cache.New(time.Minute, time.Minute)
)

func ContextSession(ctx context.Context) *discordgo.Session {
	return ctx.Value(ContextKeySession).(*discordgo.Session)
}

func ContextRedis(ctx context.Context) *redis.Client {
	return ctx.Value(common.ContextKeyRedis).(*redis.Client)
}

func RedisWrapper(inner Handler) Handler {
	return func(ctx context.Context, evt interface{}) {
		r, err := common.RedisPool.Get()
		if err != nil {
			logrus.WithError(err).WithField("evt", "{{.}}").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			common.RedisPool.Put(r)
		}()

		inner(context.WithValue(ctx, common.ContextKeyRedis, r), evt)
	}
}

func GetCreatePrivateChannel(user string) (*discordgo.Channel, error) {

	State.RLock()
	defer State.RUnlock()
	for _, c := range State.PrivateChannels {
		if c.Recipient().ID == user {
			return c.Copy(true, false), nil
		}
	}

	channel, err := common.BotSession.UserChannelCreate(user)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func SendDM(user string, msg string) error {
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

func GetMember(guildID, userID string) (*discordgo.Member, error) {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	cop := gs.MemberCopy(true, userID, true)
	if cop != nil {
		return cop, nil
	}

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		return nil, err
	}

	gs.MemberAddUpdate(true, member)

	return member, nil
}
