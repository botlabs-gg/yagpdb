package configstore

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix.v2/redis"
	"golang.org/x/net/context"
	"reflect"
	"strconv"
	"time"
)

var (
	ErrNotFound      = errors.New("Config not found")
	ErrInvalidConfig = errors.New("Invalid config")

	SQL      = &Postgres{}
	Redis    = &redisDatabase{}
	Cached   = NewCached()
	storages = make(map[reflect.Type]Storage)
)

func RegisterConfig(stor Storage, conf GuildConfig) {
	storages[reflect.TypeOf(conf)] = stor
}

func KeyGuildConfig(guildID string, configName string) string {
	return "guild_config:" + configName + ":" + guildID
}

type GuildConfigModel struct {
	GuildID   int64 `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (gm *GuildConfigModel) GetUpdatedAt() time.Time {
	return gm.UpdatedAt
}

func (gm *GuildConfigModel) GetGuildID() int64 {
	return gm.GuildID
}

type GuildConfig interface {
	GetGuildID() int64
	GetUpdatedAt() time.Time
	GetName() string
}

type PostFetchHandler interface {
	// Called after retrieving from underlying storage, before being put in cache
	// use this for any post processing etc..
	PostFetch()
}

type Storage interface {
	// GetGuildConfig returns a GuildConfig item from db
	GetGuildConfig(ctx context.Context, guildID string, dest GuildConfig) (err error)

	// SetGuildConfig saves the GuildConfig struct
	SetGuildConfig(ctx context.Context, conf GuildConfig) error

	// SetIfLatest saves it only if the passedLatest time is the latest version
	// SetIfLatest(ctx context.Context, conf GuildConfig) (updated bool, err error)
}

type CachedStorage struct {
	cache *ccache.Cache
}

func NewCached() *CachedStorage {
	return &CachedStorage{
		cache: ccache.New(ccache.Configure()),
	}
}

func (c *CachedStorage) InvalidateCache(guildID string, config string) {
	c.cache.Delete(KeyGuildConfig(guildID, config))
}

func (c *CachedStorage) GetGuildConfig(ctx context.Context, guildID string, dest GuildConfig) error {
	cached := true
	item, err := c.cache.Fetch(KeyGuildConfig(guildID, dest.GetName()), time.Minute*10, func() (interface{}, error) {
		underlying, ok := storages[reflect.TypeOf(dest)]
		if !ok {
			return nil, ErrInvalidConfig
		}

		cached = false
		err := underlying.GetGuildConfig(ctx, guildID, dest)
		if err == gorm.ErrRecordNotFound {
			err = ErrNotFound
		}

		// Call the postfetchhandler
		if err == nil {
			if pfh, ok := dest.(PostFetchHandler); ok {
				pfh.PostFetch()
			}
		}

		return dest, err
	})

	// If it was loaded from cache, we need to load it into "dest" ourselves
	if err == nil && cached {
		reflect.Indirect(reflect.ValueOf(dest)).Set(reflect.Indirect(reflect.ValueOf(item.Value())))
	}

	return err
}

func InitDatabases() {
	pubsub.AddHandler("invalidate_guild_config_cache", HandleInvalidateCacheEvt, "")
}

func HandleInvalidateCacheEvt(event *pubsub.Event) {
	conf, ok := event.Data.(*string)
	if !ok {
		logrus.Error("Invalid invalidate guild config cache event")
		return
	}

	Cached.InvalidateCache(event.TargetGuild, *conf)
}

// InvalidateGuildCache is a helper that both instantly invalides the local application cache
// As well as sending the pusub event
func InvalidateGuildCache(client *redis.Client, guildID interface{}, conf GuildConfig) {
	gStr := ""
	switch t := guildID.(type) {
	case int64:
		gStr = strconv.FormatInt(t, 10)
	case string:
		gStr = t
	case GuildConfig:
		idInt := t.GetGuildID()
		gStr = strconv.FormatInt(idInt, 10)
	default:
		panic("Invalid guildID passed to InvalidateGuildCache")
	}

	Cached.InvalidateCache(gStr, conf.GetName())
	err := pubsub.Publish(client, "invalidate_guild_config_cache", gStr, conf.GetName())
	if err != nil {
		logrus.WithError(err).Error("FAILED INVALIDATING CACHE")
	}
}
