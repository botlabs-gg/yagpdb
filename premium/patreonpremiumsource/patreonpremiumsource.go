package patreonpremiumsource

import (
	"context"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/patreon"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/premium/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

type PremiumSource struct{}

func RegisterPlugin() {
	logger.Info("Registered patreon premium source")
	premium.RegisterPremiumSource(&PremiumSource{})
	common.RegisterPlugin(&Plugin{})
}

func (ps *PremiumSource) Init() {}

func (ps *PremiumSource) Names() (human string, idname string) {
	return "Patreon", "patreon"
}

var logger = common.GetPluginLogger(&Plugin{})

// Since we keep the patrons loaded on the webserver, we also do the updating of the patreon premium slots on the webserver
// in the future this should be cleaned up (maybe keep patrons in redis)
type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Patreon premium source",
		SysName:  "patreon_premium_source",
		Category: common.PluginCategoryCore,
	}
}

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	if patreon.ActivePoller == nil {
		return
	}

	go RunPoller()
}

func RunPoller() {
	ticker := time.NewTicker(time.Minute)

	for {
		<-ticker.C
		err := UpdatePremiumSlots(context.Background())
		if err != nil {
			logger.WithError(err).Error("Failed updating premium slots for patrons")
		}
	}
}

func UpdatePremiumSlots(ctx context.Context) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	slots, err := models.PremiumSlots(qm.Where("source='patreon'"), qm.OrderBy("id desc")).All(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "PremiumSlots")
	}

	patrons := patreon.ActivePoller.GetPatrons()
	if len(patrons) == 0 {
		return nil
	}

	// Sort the slots into a map of users -> slots
	sorted := make(map[int64][]*models.PremiumSlot)

	for _, slot := range slots {
		sorted[slot.UserID] = append(sorted[slot.UserID], slot)
	}

	// Update already tracked slots
	for userID, userSlots := range sorted {
		slotsForPledge := 0
		for _, patron := range patrons {
			if patron.DiscordID == userID {
				slotsForPledge = CalcSlotsForPledge(patron.AmountCents)
				break
			}
		}

		if slotsForPledge == len(userSlots) {
			// Correct number of slots already set up
			continue
		}

		if slotsForPledge > len(userSlots) {
			// Need to create more slots
			for i := 0; i < slotsForPledge-len(userSlots); i++ {
				title := fmt.Sprintf("Patreon Slot #%d", i+len(userSlots))
				slot, err := premium.CreatePremiumSlot(ctx, tx, userID, "patreon", title, "Slot is available for as long as the pledge is active on patreon", int64(i+len(userSlots)), -1)
				if err != nil {
					tx.Rollback()
					return errors.WithMessage(err, "CreatePremiumSlot")
				}

				logger.Info("Created patreon premium slot #", slot.ID, slot.UserID)
			}
		} else if slotsForPledge < len(userSlots) {
			// Need to remove slots
			slotsToRemove := make([]int64, 0)

			for i := 0; i < len(userSlots)-slotsForPledge; i++ {
				slot := userSlots[i]
				slotsToRemove = append(slotsToRemove, slot.ID)
				logger.Info("Marked patreon slot for deletion #", slot.ID, slot.UserID)
			}

			err = premium.RemovePremiumSlots(ctx, tx, userID, slotsToRemove)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "RemovePremiumSlots")
			}
		}
	}

	// Add completely new premium slots
OUTER:
	for _, v := range patrons {
		if v.DiscordID == 0 {
			continue
		}

		for userID, _ := range sorted {
			if userID == v.DiscordID {
				continue OUTER
			}
		}

		// If we got here then that means this is a new user
		slots := CalcSlotsForPledge(v.AmountCents)
		for i := 0; i < slots; i++ {
			title := fmt.Sprintf("Patreon Slot #%d", i+1)
			slot, err := premium.CreatePremiumSlot(ctx, tx, v.DiscordID, "patreon", title, "Slot is available for as long as the pledge is active on patreon", int64(i+1), -1)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "new CreatePremiumSlot")
			}

			logger.Info("Created new patreon premium slot #", slot.ID, slot.ID)
		}
	}

	err = tx.Commit()
	return errors.WithMessage(err, "Commit")
}

func CalcSlotsForPledge(cents int) (slots int) {
	if cents < 300 {
		return 0
	}

	// 3$ for one slot
	if cents >= 300 && cents < 500 {
		return 1
	}

	// 2.5$ per slot up until before 10$
	if cents < 1000 {
		return cents / 250
	}

	// 2$ per slot from and including 10$
	return cents / 200
}
