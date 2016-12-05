package configstore

import (
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/net/context"
)

type Postgres struct{}

// conf is requried to be a pointer value
func (p *Postgres) GetGuildConfig(ctx context.Context, guildID string, conf GuildConfig) error {
	err := common.SQL.Where("guild_id = ?", guildID).First(conf).Error
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

	redisClient, err := RedisClientCtx(ctx)
	if err != nil {
		return err
	}

	InvalidateGuildCache(redisClient, conf, conf)
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
		InvalidateGuildCache(redisClient, conf, conf)
	}

	return
}
