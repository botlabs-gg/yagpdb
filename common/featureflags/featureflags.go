package featureflags

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PluginWithFeatureFlags is a interface for plugins that provide their own feature-flags
type PluginWithFeatureFlags interface {
	common.Plugin

	UpdateFeatureFlags(guildID int64) ([]string, error)
	AllFeatureFlags() []string
}

// this provides a way to batch update flags initially during init
// for example with premium, we can get all the premium guilds in 1 db query
// as opposed to querying each individual guild individually
type PluginWithBatchFeatureFlags interface {
	PluginWithFeatureFlags

	UpdateFeatureFlagsBatch() (map[int64][]string, error)
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

func (c *flagCache) initCacheBatch(guilds []int64) error {
	c.l.Lock()
	defer c.l.Unlock()

	actions := make([]radix.CmdAction, 0, len(guilds))
	results := make([][]string, 0, len(guilds))
	fetchingGuilds := make([]int64, 0, len(guilds))
	i := 0
	for _, g := range guilds {
		if _, ok := c.cache[g]; ok {
			continue // already in cache
		}

		results = append(results, make([]string, 0))
		// results[g] = make([]string, 0)
		actions = append(actions, radix.Cmd(&results[i], "SMEMBERS", keyGuildFlags(g)))
		fetchingGuilds = append(fetchingGuilds, g)

		i++
	}

	if len(actions) < 1 {
		return nil
	}

	err := common.RedisPool.Do(radix.Pipeline(actions...))
	if err != nil {
		return err
	}

	for i, g := range fetchingGuilds {
		c.cache[g] = results[i]
	}
	return nil
}

func (c *flagCache) invalidateGuild(guildID int64) {
	c.l.Lock()
	defer c.l.Unlock()

	delete(c.cache, guildID)
}

var (
	caches = initCaches()
)

func BatchInitCache(guilds []int64) error {
	logger.Infof("started preloading flag cache for %d guilds", len(guilds))
	started := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < len(caches); i++ {
		toFetchHere := make([]int64, 0, len(guilds))

		for _, g := range guilds {
			cacheID := (g >> 22) % int64(len(caches))
			if cacheID == int64(i) {
				toFetchHere = append(toFetchHere, g)
			}
		}
		wg.Add(1)
		go func(cacheID int, guildsToFetch []int64) {
			defer wg.Done()

			err := caches[cacheID].initCacheBatch(toFetchHere)
			if err != nil {
				logger.WithError(err).Error("failed preloading flag cache")
			}
		}(i, toFetchHere)
	}

	wg.Wait()
	logger.Infof("Preloading flag cache done, dur: %s", time.Since(started))
	return nil
}

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

var metricsFeatureFlagsUpdated = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_featureflags_updated_guilds_total",
	Help: "Guilds featureflags has been updated for",
})

const evictCachePubSubEvent = "feature_flags_updated"
const evictCachePubSubEvent2 = "feature_flags_updated_2"

// UpdateGuildFlags updates the provided guilds feature flags
func UpdateGuildFlags(guildID int64) error {
	defer pubsub.Publish(evictCachePubSubEvent, guildID, nil)
	defer pubsub.Publish(evictCachePubSubEvent2, -1, EvictCacheData{GuildID: guildID})

	var lastErr error
	for _, p := range common.Plugins {
		if cast, ok := p.(PluginWithFeatureFlags); ok {
			err := updatePluginFeatureFlags(guildID, cast)
			if err != nil {
				lastErr = err
			}
		}
	}

	metricsFeatureFlagsUpdated.Inc()

	return lastErr
}

// UpdatePluginFeatureFlags updates the feature flags of the provided plugin for the provided guild
func UpdatePluginFeatureFlags(guildID int64, p PluginWithFeatureFlags) error {
	defer EvictCacheForGuild(guildID)
	defer pubsub.Publish(evictCachePubSubEvent, guildID, nil)
	defer pubsub.Publish(evictCachePubSubEvent2, -1, EvictCacheData{GuildID: guildID})

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

// in some scenarios manual flag management is usefull and since updating flags
// dosen't trample over unknown flags its completely reliable aswelll
func AddManualGuildFlags(guildID int64, flags ...string) error {
	err := common.RedisPool.Do(radix.Cmd(nil, "SADD", append([]string{keyGuildFlags(guildID)}, flags...)...))
	if err == nil {
		pubsub.PublishLogErr(evictCachePubSubEvent, guildID, nil)
		pubsub.PublishLogErr(evictCachePubSubEvent2, -1, EvictCacheData{GuildID: guildID})
	}

	return err
}

// in some scenarios manual flag management is usefull and since updating flags
// dosen't trample over unknown flags its completely reliable aswelll
func RemoveManualGuildFlags(guildID int64, flags ...string) error {
	err := common.RedisPool.Do(radix.Cmd(nil, "SREM", append([]string{keyGuildFlags(guildID)}, flags...)...))
	if err == nil {
		pubsub.PublishLogErr(evictCachePubSubEvent, guildID, nil)
		pubsub.PublishLogErr(evictCachePubSubEvent2, -1, EvictCacheData{GuildID: guildID})
	}

	return err
}
