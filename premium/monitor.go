package premium

import (
	"context"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	go runMonitor()
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Done()
}

func runMonitor() {
	ticker := time.NewTicker(time.Second * 30)
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
			// remove any stale redis entries and trigger removal events
			if err := syncPremiumServers(context.Background()); err != nil {
				logger.WithError(err).Error("Failed syncing premium servers (cleanup)")
			}

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
func syncPremiumServers(ctx context.Context) error {
	// Read all current premium guild IDs from redis
	var redisGuildIDs []int64
	logger.Info("Premium Server Sync: Getting guild IDs from redis")
	if err := common.RedisPool.Do(radix.Cmd(&redisGuildIDs, "HKEYS", RedisKeyPremiumGuilds)); err != nil {
		return errors.WithMessage(err, "hkeys premium guilds")
	}

	if len(redisGuildIDs) == 0 {
		logger.Info("Premium Server Sync: No premium servers found in redis, skipping sync")
		return nil
	}

	// Build a set of DB guilds
	slots, err := models.PremiumSlots(qm.Where("guild_id IS NOT NULL")).AllG(ctx)
	if err != nil {
		logger.WithError(err).Error("Premium Server Sync: Failed getting premium slots from database")
		return errors.WithMessage(err, "models.PremiumSlots")
	}
	logger.Infof("Premium Server Sync: Redis Slots: %d, DB Slots: %d", len(redisGuildIDs), len(slots))
	dbGuilds := make(map[int64]struct{}, len(slots))
	for _, s := range slots {
		dbGuilds[s.GuildID.Int64] = struct{}{}
	}

	// For each redis guild not present in DB, remove and emit event
	for _, guildID := range redisGuildIDs {
		// if the guild is still premium, skip
		if _, ok := dbGuilds[guildID]; ok {
			continue
		}

		logger.Infof("Premium Server Sync: Removing guild %d from redis", guildID)
		// Remove from redis
		if err := common.RedisPool.Do(radix.FlatCmd(nil, "HDEL", RedisKeyPremiumGuilds, guildID)); err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Premium Server Sync: Failed HDEL stale premium guild")
			continue
			// continue attempting others
		}

		// trigger removal event to disable premium features
		if err := scheduledevents2.ScheduleEvent("premium_guild_removed", guildID, time.Now(), nil); err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Premium Server Sync: Failed scheduling premium_guild_removed")
			continue
		}
	}

	return nil
}

// Updates ALL premiun slots from ALL sources
func checkExpiredSlots(ctx context.Context) error {
	timedSlots, err := models.PremiumSlots(qm.Where("permanent = false"), qm.Where("guild_id IS NOT NULL")).AllG(ctx)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}

	for _, v := range timedSlots {
		if SlotDurationLeft(v) <= 0 {
			err := SlotExpired(ctx, v)
			if err != nil {
				logger.WithError(err).WithField("slot", v.ID).Error("Failed expiring premium slot")
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
