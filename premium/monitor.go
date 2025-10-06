package premium

import (
	"context"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	syncPremiumServersOnStart()
	go runMonitor()
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Done()
}

func runMonitor() {
	ticker := time.NewTicker(time.Minute)
	time.Sleep(time.Second * 3)
	logger.Info("started premium server monitor")

	err := checkExpiredSlots(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed checking for expired premium slots")
	}

	checkedExpiredSlots := false
	for {
		<-ticker.C
		if checkedExpiredSlots {
			// Then, refresh the redis cache from DB
			err := updatePremiumServers(context.Background())
			if err != nil {
				logger.WithError(err).Error("Failed updating premium servers")
			}
			checkedExpiredSlots = false
		} else {
			err := checkExpiredSlots(context.Background())
			if err != nil {
				logger.WithError(err).Error("Failed checking for expired premium slots")
			}
			checkedExpiredSlots = true
		}

	}
}

// This syncs servers between Redis and the database and removes any guilds present in Redis but not in DB.
// For each removed guild, it schedules the premium_guild_removed event and updates feature flags.
func syncPremiumServersOnStart() error {
	logger.Info("Premium Server Sync: Getting All Guilds Once Premium")
	allOncePremiumGuildIDs, err := AllGuildsOncePremium()
	if err != nil {
		logger.Info("Premium Server Sync: Failed getting all guild once premium")
		return errors.WithMessage(err, "AllGuildsOncePremium")
	}

	// Build a set of DB guilds
	slots, err := models.PremiumSlots(qm.Where("guild_id IS NOT NULL")).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("Premium Server Sync: Failed getting premium slots from database")
		return errors.WithMessage(err, "models.PremiumSlots")
	}
	logger.Info("Premium Server Sync: Getting All Guilds Currently Premium from Database")
	dbGuilds := make(map[int64]struct{}, len(slots))
	for _, s := range slots {
		dbGuilds[s.GuildID.Int64] = struct{}{}
	}

	logger.Infof("Premium Server Sync: All Guilds once premium: %d, Current Premium Guilds: %d", len(allOncePremiumGuildIDs), len(dbGuilds))
	// For each redis guild not present in DB, remove and emit event
	for guildID := range allOncePremiumGuildIDs {
		// if the guild is still premium, skip
		if _, ok := dbGuilds[guildID]; ok {
			continue
		}

		isPremium, err := IsGuildPremium(guildID)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Failed checking if guild is premium")
			continue
		}
		if !isPremium {
			continue
		}
		logger.Infof("Premium Server Sync: Removing premium feature flag for guild %d", guildID)
		// Remove from redis
		if err := common.RedisPool.Do(radix.FlatCmd(nil, "HDEL", RedisKeyPremiumGuilds, guildID)); err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Premium Server Sync: Failed HDEL stale premium guild")
			continue
		}

		err = featureflags.UpdatePluginFeatureFlags(guildID, &Plugin{})
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Premium Server Sync: Failed updating plugin feature flags")
			continue
		}

		// trigger removal event to disable premium features
		if err := scheduledevents2.ScheduleEvent("premium_guild_removed", guildID, time.Now(), nil); err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Premium Server Sync: Failed scheduling premium_guild_removed")
			continue
		}

	}

	for _, slot := range slots {
		isPremium, err := IsGuildPremium(slot.GuildID.Int64)
		if err != nil {
			logger.WithError(err).WithField("guild", slot.GuildID.Int64).Error("Failed checking if guild is premium")
			continue
		}
		if isPremium {
			continue
		}
		logger.Infof("Premium Server Sync: guild %d should've been premium, correcting", slot.GuildID.Int64)
		err = common.RedisPool.Do(radix.FlatCmd(nil, "HSET", RedisKeyPremiumGuilds, slot.GuildID.Int64, slot.UserID))
		if err != nil {
			logger.WithError(err).WithField("guild", slot.GuildID.Int64).Errorf("Premium Server Sync: Failed setting guild in redis")
			continue
		}
		err = scheduledevents2.ScheduleEvent("premium_guild_added", slot.GuildID.Int64, time.Now(), nil)
		if err != nil {
			logger.WithError(err).WithField("guild", slot.GuildID.Int64).Errorf("Premium Server Sync: Failed triggering premium_guild_added")
			continue
		}
		err = featureflags.UpdatePluginFeatureFlags(slot.GuildID.Int64, &Plugin{})
		if err != nil {
			logger.WithError(err).WithField("guild", slot.GuildID.Int64).Errorf("Premium Server Sync: Failed updating plugin feature flags")
			continue
		}
	}

	return nil
}

// Updates ALL premiun slots from ALL sources
func checkExpiredSlots(ctx context.Context) error {
	timedSlots, err := models.PremiumSlots(qm.Where("permanent = false")).AllG(ctx)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}

	for _, v := range timedSlots {
		if SlotDurationLeft(v) <= 0 {
			tx, err := common.PQ.BeginTx(ctx, nil)
			if err != nil {
				return errors.WithMessage(err, "BeginTX")
			}
			err = RemovePremiumSlots(ctx, tx, v.UserID, []int64{v.ID})
			if err != nil {
				tx.Rollback()
				logger.WithError(err).WithField("slot", v.ID).Error("Failed removing expired premium slot")
			}
			err = tx.Commit()
			if err != nil {
				return errors.WithMessage(err, "Commit")
			}
		}
	}

	return nil
}

func updatePremiumServers(ctx context.Context) error {
	slots, err := models.PremiumSlots(qm.Where("guild_id IS NOT NULL")).AllG(ctx)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}

	if len(slots) < 1 {
		// Fast path
		err = common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyPremiumGuilds))
		return errors.WithMessage(err, "do.Del")
	}

	rCmd := []string{RedisKeyPremiumGuildsTmp}
	rCmdLastTimesPremium := []string{RedisKeyPremiumGuildLastActive}
	now := strconv.FormatInt(time.Now().Unix(), 10)

	for _, slot := range slots {
		strGID := strconv.FormatInt(slot.GuildID.Int64, 10)
		strUserID := strconv.FormatInt(slot.UserID, 10)

		rCmd = append(rCmd, strGID, strUserID)
		rCmdLastTimesPremium = append(rCmdLastTimesPremium, now, strGID)
	}

	if err = common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyPremiumGuildsTmp)); err != nil {
		return errors.WithMessage(err, "del tmp")
	}
	if err = common.RedisPool.Do(radix.Cmd(nil, "HMSET", rCmd...)); err != nil {
		return errors.WithMessage(err, "hmset")
	}
	if err = common.RedisPool.Do(radix.Cmd(nil, "RENAME", RedisKeyPremiumGuildsTmp, RedisKeyPremiumGuilds)); err != nil {
		return errors.WithMessage(err, "rename")
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "ZADD", rCmdLastTimesPremium...))
	return errors.WithMessage(err, "last_premium_times")
}
