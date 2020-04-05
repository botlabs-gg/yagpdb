package premium

import (
	"context"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/premium/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/queries/qm"
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
