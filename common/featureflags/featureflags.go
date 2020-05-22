package featureflags

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/mediocregopher/radix/v3"
)

// PluginWithFeatureFlags is a interface for plugins that provide their own feature-flags
type PluginWithFeatureFlags interface {
	common.Plugin

	UpdateFeatureFlags(guildID int64) ([]string, error)
	AllFeatureFlags() []string
}

func keyGuildFlags(guildID int64) string {
	return fmt.Sprintf("f_flags:%d", guildID)
}

type flagCache struct {
	cache map[int64][]string
	l     sync.RWMutex
}

func initCaches() []*flagCache {
	result := make([]*flagCache, 10)
	for i, _ := range result {
		result[i] = &flagCache{
			cache: make(map[int64][]string),
		}
	}

	return result
}

// GetGuildFlags returns the feature flags a guild has
func (c *flagCache) getGuildFlags(guildID int64) ([]string, error) {
	// fast path
	c.l.RLock()
	if flags, ok := c.cache[guildID]; ok {
		c.l.RUnlock()
		return flags, nil
	}

	c.l.RUnlock()

	// need to fetch from redis, upgrade lock
	c.l.Lock()
	defer c.l.Unlock()

	// check again in case in the mean time we got the flags while trying to upgrade the lock
	if flags, ok := c.cache[guildID]; ok {
		// the flags for this server was fetched in the meantime
		return flags, nil
	}

	var result []string
	err := common.RedisPool.Do(radix.Cmd(&result, "SMEMBERS", keyGuildFlags(guildID)))
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	c.cache[guildID] = result
	return result, nil
}

func (c *flagCache) invalidateGuild(guildID int64) {
	c.l.Lock()
	defer c.l.Unlock()

	delete(c.cache, guildID)
}

var (
	caches = initCaches()
)

// GetGuildFlags returns the feature flags a guild has
func GetGuildFlags(guildID int64) ([]string, error) {
	cacheID := (guildID >> 22) % int64(len(caches))
	return caches[cacheID].getGuildFlags(guildID)
}

// RetryGetGuildFlags is the same as GetGuildFlags but will retry fetching the flags up to around 2 minutes before giving up on errors
func RetryGetGuildFlags(guildID int64) (flags []string, err error) {
	maxRetry := 120

	for i := 0; i < maxRetry; i++ {
		flags, err = GetGuildFlags(guildID)
		if err != nil {
			logger.WithError(err).Error("failed retrieving flags, trying again...")
			time.Sleep(time.Second)
			continue
		}

		return flags, nil
	}

	logger.Error("Gave up trying to fetch feature flags")
	return
}

// GuildHasFlag returns true if the target guild has the provided flag
func GuildHasFlag(guildID int64, flag string) (bool, error) {
	flags, err := GetGuildFlags(guildID)
	if err != nil {
		return false, err
	}

	return common.ContainsStringSlice(flags, flag), nil
}

// GuildHasFlagOrLogError is the same as GuildHasFlag but will handle the error and log it
func GuildHasFlagOrLogError(guildID int64, flag string) bool {
	hasFlag, err := GuildHasFlag(guildID, flag)
	if err != nil {
		logger.WithError(err).Errorf("failed checking feature flag %+v", err)
		return false
	}

	return hasFlag
}

const evictCachePubSubEvent = "feature_flags_updated"

// UpdateGuildFlags updates the provided guilds feature flags
func UpdateGuildFlags(guildID int64) error {
	keyLock := fmt.Sprintf("feature_flags_updating:%d", guildID)
	err := common.BlockingLockRedisKey(keyLock, time.Second*60, 60)
	if err != nil {
		return errors.WithStackIf(err)
	}

	defer common.UnlockRedisKey(keyLock)
	defer pubsub.Publish(evictCachePubSubEvent, guildID, nil)

	var lastErr error
	for _, p := range common.Plugins {
		if cast, ok := p.(PluginWithFeatureFlags); ok {
			err := updatePluginFeatureFlags(guildID, cast)
			if err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

// UpdatePluginFeatureFlags updates the feature flags of the provided plugin for the provided guild
func UpdatePluginFeatureFlags(guildID int64, p PluginWithFeatureFlags) error {
	defer pubsub.Publish(evictCachePubSubEvent, guildID, nil)
	return updatePluginFeatureFlags(guildID, p)
}

func updatePluginFeatureFlags(guildID int64, p PluginWithFeatureFlags) error {

	allFlags := p.AllFeatureFlags()

	activeFlags, err := p.UpdateFeatureFlags(guildID)
	if err != nil {
		return errors.WithStackIf(err)
	}

	toDel := make([]string, 0)
	for _, v := range allFlags {
		if common.ContainsStringSlice(activeFlags, v) {
			continue
		}

		// flag isn't active
		toDel = append(toDel, v)
	}

	filtered := make([]string, 0, len(activeFlags))

	// make sure all flags are valid
	for _, v := range activeFlags {
		if !common.ContainsStringSlice(allFlags, v) {
			logger.WithError(err).Errorf("Flag %q is not in the spec of %s", v, p.PluginInfo().SysName)
		} else {
			filtered = append(filtered, v)
		}
	}

	key := keyGuildFlags(guildID)

	err = common.RedisPool.Do(radix.WithConn(key, func(conn radix.Conn) error {

		// apply the added/unchanged flags first
		if len(filtered) > 0 {
			err := conn.Do(radix.Cmd(nil, "SADD", append([]string{key}, filtered...)...))
			if err != nil {
				return errors.WithStackIf(err)
			}
		}

		// then remove the flags we don't have
		if len(toDel) > 0 {
			err = conn.Do(radix.Cmd(nil, "SREM", append([]string{key}, toDel...)...))
			if err != nil {
				return errors.WithStackIf(err)
			}
		}

		return nil
	}))

	if err != nil {
		return errors.WithStackIf(err)
	}

	return nil
}
