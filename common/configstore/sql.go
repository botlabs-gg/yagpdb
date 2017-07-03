package configstore

import (
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/net/context"
	"math/rand"
	"strings"
	"time"
)

const MaxRetries = 1000

type Postgres struct{}

// conf is requried to be a pointer value
func (p *Postgres) GetGuildConfig(ctx context.Context, guildID string, conf GuildConfig) error {

	currentRetries := 0
	for {

		err := common.GORM.Where("guild_id = ?", guildID).First(conf).Error
		if err == nil {
			if currentRetries > 1 {
				logrus.Info("Suceeded after ", currentRetries, " retries")
			}
			return nil
		}

		if err == gorm.ErrRecordNotFound {
			return ErrNotFound
		}

		if strings.Contains(err.Error(), "sorry, too many clients already") {
			time.Sleep(time.Millisecond * 10 * time.Duration(rand.Intn(10)))
			currentRetries++
			if currentRetries > MaxRetries {
				return err
			}
			continue
		}

		return err
	}

	return nil
}

// conf is requried to be a pointer value
func (p *Postgres) SetGuildConfig(ctx context.Context, conf GuildConfig) error {
	err := common.GORM.Save(conf).Error
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
	result := common.GORM.Where("updated_at = ?", conf.GetUpdatedAt()).Save(conf)
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
