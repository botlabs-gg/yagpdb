package featureflags

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
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

	needsFullRefresh := false
	for _, v := range common.Plugins {
		if plugin, ok := v.(PluginWithFeatureFlags); ok {
			pluginFlags := plugin.AllFeatureFlags()
			for _, v := range pluginFlags {
				if !common.ContainsStringSlice(currentInitFlags, v) {
					// NEW FLAG! Refresh needed...
					needsFullRefresh = true
					newFlags = append(newFlags, v)
				}
			}
		}
	}

	if !needsFullRefresh {
		logger.Info("no new featue flags detected...")
		return
	}

	logger.Infof("NEW FEATURE FLAGS DETECTED! Full refresh is needed: %v", newFlags)
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

// periodically checks if were storing feature flags of some servers that has left, and also checks for missing feature flags
func (p *Plugin) runLeftGuildsCheck() error {
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
