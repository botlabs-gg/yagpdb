package configstore

import (
	"encoding/json"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/mediocregopher/radix.v2/redis"
	"golang.org/x/net/context"
	"strconv"
)

func RedisClientCtx(ctx context.Context) (*redis.Client, error) {
	if client := ctx.Value(common.ContextKeyRedis); client != nil {
		return client.(*redis.Client), nil
	}

	return common.RedisPool.Get()
}

// Redis database has no last update protection
type redisDatabase struct{}

func (r *redisDatabase) GetGuildConfig(ctx context.Context, guildID string, conf GuildConfig) error {
	client, err := RedisClientCtx(ctx)
	if err != nil {
		return err
	}

	reply := client.Cmd("GET", KeyGuildConfig(guildID, conf.GetName()))
	if reply.IsType(redis.Nil) {
		return ErrNotFound
	}

	data, err := reply.Bytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, conf)
	return err
}

func (r *redisDatabase) SetGuildConfig(ctx context.Context, conf GuildConfig) error {
	client, err := RedisClientCtx(ctx)
	if err != nil {
		return err
	}

	guildIDStr := strconv.FormatInt(conf.GetGuildID(), 10)

	err = common.SetRedisJson(client, KeyGuildConfig(guildIDStr, conf.GetName()), conf)
	if err != nil {
		return err
	}

	pubsub.Publish(client, "invalidate_guild_config_cache", guildIDStr, conf.GetName())
	return nil
}

// SetIfLatest saves it only if the passedLatest time is the latest version
func (r *redisDatabase) SetIfLatest(ctx context.Context, conf GuildConfig) (updated bool, err error) {
	err = r.SetGuildConfig(ctx, conf)
	return true, err
}
