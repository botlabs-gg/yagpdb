package configstore

import (
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"golang.org/x/net/context"
	"strconv"
)

type Postgres struct{}

// conf is requried to be a pointer value
func (p *Postgres) GetGuildConfig(ctx context.Context, guildID string, conf GuildConfig) error {
	err := common.SQL.Where(guildID).First(conf).Error
	if err == gorm.ErrRecordNotFound {
		return ErrNotFound
	}
	return err
}

// conf is requried to be a pointer value
func (p *Postgres) SetGuildConfig(ctx context.Context, conf GuildConfig) error {
	err := common.SQL.Save(conf).Error
	if err != nil {
		return err
	}

	strGuildID := strconv.FormatInt(conf.GetGuildID(), 10)

	redisClient, err := RedisClientCtx(ctx)
	if err != nil {
		return err
	}

	pubsub.Publish(redisClient, "invalidate_guild_config_cache", strGuildID, conf.GetName())
	return nil
}

func (p *Postgres) SetIfLatest(ctx context.Context, conf GuildConfig) (updated bool, err error) {
	result := common.SQL.Where("updated_at = ?", conf.GetUpdatedAt()).Save(conf)
	updated = result.RowsAffected > 0
	err = result.Error

	if err == nil {
		redisClient, err := RedisClientCtx(ctx)
		if err != nil {
			return false, err
		}
		strGuildID := strconv.FormatInt(conf.GetGuildID(), 10)
		pubsub.Publish(redisClient, "invalidate_guild_config_cache", strGuildID, conf.GetName())
	}

	return
}
