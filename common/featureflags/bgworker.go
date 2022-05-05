package featureflags

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/mediocregopher/radix/v3"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

// RunBackgroundWorker implements backgroundworkers.BackgroundWorkerPlugin
func (p *Plugin) RunBackgroundWorker() {
	p.checkInitFeatureFlags()

	t := time.NewTicker(time.Second)
	for {
		select {
		case <-t.C:
		case wg := <-p.stopBGWorker:
			wg.Done()
			return
		}

		err := p.runUpdateDirtyFlags()
		if err != nil {
			logger.WithError(err).Errorf("Failed updating dirty flags %+v", err)
		}
	}
}

// StopBackgroundWorker implements backgroundworkers.BackgroundWorkerPlugin
func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	p.stopBGWorker <- wg
}

// checks if theres new feature flags which needs to be initialized
func (p *Plugin) checkInitFeatureFlags() {
	var currentInitFlags []string
	err := common.RedisPool.Do(radix.Cmd(&currentInitFlags, "SMEMBERS", "feature_flags_initialized"))
	if err != nil {
		panic(fmt.Sprintf("Failed intializing feature flags, failed retreiving old intiailized feature-flags: %v", err))
	}

	var newFlags []string
	var newFlagsPlugins []PluginWithFeatureFlags

	for _, v := range common.Plugins {
		if plugin, ok := v.(PluginWithFeatureFlags); ok {
			pluginFlags := plugin.AllFeatureFlags()
			pluginHadMissingFlags := false

			for _, v := range pluginFlags {
				if !common.ContainsStringSlice(currentInitFlags, v) {
					// NEW FLAG! Refresh needed...
					newFlags = append(newFlags, v)
					pluginHadMissingFlags = true
				}
			}

			if pluginHadMissingFlags {
				newFlagsPlugins = append(newFlagsPlugins, plugin)
			}
		}
	}

	if len(newFlags) == 0 {
		logger.Info("no new featue flags detected...")
		return
	}

	logger.Infof("NEW FEATURE FLAGS DETECTED: %v", newFlags)
	for _, v := range newFlagsPlugins {
		logger.Infof("Plugin %s has new feature flags.", v.PluginInfo().Name)
	}

	if len(newFlagsPlugins) == 1 {
		if batchUpdater, ok := newFlagsPlugins[0].(PluginWithBatchFeatureFlags); ok {
			logger.Info("Plugin is a batch updater, trying to initially fast batch update the feature flags")
			err = p.BatchInitialPluginUpdater(batchUpdater)
			if err != nil {
				panic("Failed to batch update feature flags, falling back to legacy full update")
			}

			// mark all the new plugins as intialized
			err = common.RedisPool.Do(radix.Cmd(nil, "SADD", append([]string{"feature_flags_initialized"}, newFlags...)...))
			if err != nil {
				panic(fmt.Sprintf("Failed intializing feature flags, failed setting new intialized feature flags: %v", err))
			}

			return
		}
	}

	// mark all guilds are dirty, but low priority as to not interrupt normal operation
	err = common.RedisPool.Do(radix.Cmd(nil, "SUNIONSTORE", "feature_flags_dirty_low_priority", "feature_flags_dirty_low_priority", "connected_guilds"))
	if err != nil {
		panic(fmt.Sprintf("Failed intializing feature flags, failed marking all guilds as dirty: %v", err))
	}

	// mark all the new plugins as intialized
	err = common.RedisPool.Do(radix.Cmd(nil, "SADD", append([]string{"feature_flags_initialized"}, newFlags...)...))
	if err != nil {
		panic(fmt.Sprintf("Failed intializing feature flags, failed setting new intialized feature flags: %v", err))
	}
}

// Does a sparse initial update for new feature flags
// This is only used in cases where the feature flags can be calculated in a batch to save time
// This does NOT remove any flags so its purely to intiially populate the flags
// It should also run fairly fast since its blocking normal operation
func (p *Plugin) BatchInitialPluginUpdater(pbf PluginWithBatchFeatureFlags) error {
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		logger.Infof("Took %s to batch update feature flags", elapsed.String())
	}()

	guildsFlagMap, err := pbf.UpdateFeatureFlagsBatch()
	if err != nil {
		return err
	}

	logger.Infof("Batch initial updating of %d guilds", len(guildsFlagMap))

	// create the redis commands to add the keys
	actions := make([]radix.CmdAction, 0, len(guildsFlagMap))
	for g, flags := range guildsFlagMap {
		key := keyGuildFlags(g)
		args := append([]string{key}, flags...)
		actions = append(actions, radix.Cmd(nil, "SADD", args...))
	}

	err = common.RedisPool.Do(radix.Pipeline(actions...))
	if err != nil {
		return err
	}

	return nil
}

// updates all dirty flags
func (p *Plugin) runUpdateDirtyFlags() (err error) {
	started := time.Now()

	list := "high priority"
	var guildIDs []int64
	if err := common.RedisPool.Do(radix.Cmd(&guildIDs, "SPOP", "feature_flags_dirty_high_priority", "25")); err != nil {
		return errors.WithStackIf(err)
	}

	if len(guildIDs) < 1 {
		list = "low priority"
		// No more high priority flags to update
		if err := common.RedisPool.Do(radix.Cmd(&guildIDs, "SPOP", "feature_flags_dirty_low_priority", "25")); err != nil {
			return errors.WithStackIf(err)
		}

		if len(guildIDs) < 1 {
			return nil
		}
	}

	for _, guildID := range guildIDs {
		err = UpdateGuildFlags(guildID)
		if err != nil {
			// re-mark this as dirty, in case there was a short interruption with the redis connection, try again until it succeeds
			go MarkGuildDirty(guildID)
			return errors.WithStackIf(err)
		}
	}

	elapsed := time.Since(started)
	logger.Infof("Updated %d %s flags in %s", len(guildIDs), list, elapsed.String())
	return nil
}

// MarkGuildDirty marks a guild's feature flags dirty
func MarkGuildDirty(guildID int64) {
	for {
		err := common.RedisPool.Do(radix.FlatCmd(nil, "SADD", "feature_flags_dirty_high_priority", guildID))
		if err == nil {
			break
		}

		logger.WithError(err).Errorf("failed marking guild dirty, trying again... %+v", err)
		time.Sleep(time.Second)
	}
}
