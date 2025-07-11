package patreonpremiumsource

import (
	"context"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/RhykerWells/yagpdb/v2/common"
	"github.com/RhykerWells/yagpdb/v2/common/patreon"
	"github.com/RhykerWells/yagpdb/v2/premium"
	"github.com/RhykerWells/yagpdb/v2/premium/models"
	"github.com/RhykerWells/yagpdb/v2/web"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
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
		if !patreon.ActivePoller.IsLastFetchSuccess() {
			logger.Warn("Last fetch was not successful, skipping update")
			continue
		}
		logger.Info("Last fetch was successful,  Updating premium slots for patrons")
		err := UpdatePatreonPremiumSlots(context.Background())
		if err != nil {
			logger.WithError(err).Error("Failed updating premium slots for patrons")
		}
	}
}

func UpdatePatreonPremiumSlots(ctx context.Context) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}
	slots, err := models.PremiumSlots(qm.Where("source=? and deletes_at IS NULL", string(premium.PremiumSourceTypePatreon)), qm.OrderBy("id desc")).All(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "PremiumSlots")
	}
	patrons := patreon.ActivePoller.GetPatrons()
	if len(patrons) == 0 {
		tx.Rollback()
		return nil
	}

	patronMap := make(map[int64]*patreon.Patron)
	for _, patron := range patrons {
		if p, ok := patronMap[patron.DiscordID]; !ok || p.AmountCents < patron.AmountCents {
			patronMap[patron.DiscordID] = patron
		}
	}

	// Sort the slots into a map of users -> slots
	sorted := make(map[int64][]*models.PremiumSlot)

	for _, slot := range slots {
		sorted[slot.UserID] = append(sorted[slot.UserID], slot)
	}

	// Update already tracked slots
	for userID, userSlots := range sorted {
		slotsForPledge := 0
		if patron, ok := patronMap[userID]; ok {
			slotsForPledge = CalcSlotsForPledge(patron.AmountCents)
		}
		if slotsForPledge == len(userSlots) {
			// Correct number of slots already set up
			continue
		}

		if slotsForPledge > len(userSlots) {
			markedForDeletion, err := premium.UserPremiumMarkedDeletedSlots(ctx, userID, premium.PremiumSourceTypePatreon)
			if err != nil {
				tx.Rollback()
				return err
			}
			newSlots := slotsForPledge - len(userSlots)
			if len(markedForDeletion) > 0 {
				err := premium.CancelSlotDeletionForUser(ctx, tx, userID, markedForDeletion)
				if err != nil {
					tx.Rollback()
					return err
				}
				newSlots -= len(markedForDeletion)
			}
			// Need to create more slots
			for i := 0; i < newSlots; i++ {
				title := fmt.Sprintf("Patreon Slot #%d", 1+i+len(userSlots))
				slot, err := premium.CreatePremiumSlot(ctx, tx, userID, premium.PremiumSourceTypePatreon, title, "Slot is available for as long as the pledge is active on patreon", int64(i+len(userSlots)), -1, premium.PremiumTierPremium)
				if err != nil {
					tx.Rollback()
					return errors.WithMessage(err, "CreatePremiumSlot")
				}
				logger.Info("Created patreon premium slot #", slot.ID, slot.UserID)
			}
			if newSlots > 0 {
				go premium.SendPremiumDM(userID, premium.PremiumSourceTypePatreon, newSlots)
			}
		} else if slotsForPledge < len(userSlots) {
			// Need to remove slots
			slotsToRemove := make([]int64, 0)

			for i := 0; i < len(userSlots)-slotsForPledge; i++ {
				slot := userSlots[i]
				slotsToRemove = append(slotsToRemove, slot.ID)
				logger.Info("Marked patreon slot for deletion #", slot.ID, slot.UserID)
			}

			err = premium.MarkSlotsForDeletion(ctx, tx, userID, slotsToRemove)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "RemovePremiumSlots")
			}
		}
		err = premium.RemoveMarkedDeletedSlots(ctx, tx, premium.PremiumSourceTypePatreon)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Add completely new premium slots
OUTER:
	for _, v := range patronMap {
		if v.DiscordID == 0 {
			continue
		}

		for userID := range sorted {
			if userID == v.DiscordID {
				continue OUTER
			}
		}

		// If we got here then that means this is a new user
		slots := CalcSlotsForPledge(v.AmountCents)
		markedForDeletion, err := premium.UserPremiumMarkedDeletedSlots(ctx, v.DiscordID, premium.PremiumSourceTypePatreon)
		if err != nil {
			tx.Rollback()
			return err
		}
		if len(markedForDeletion) > 0 {
			err := premium.CancelSlotDeletionForUser(ctx, tx, v.DiscordID, markedForDeletion)
			if err != nil {
				tx.Rollback()
				return err
			}
			slots -= len(markedForDeletion)
		}
		for i := 0; i < slots; i++ {
			title := fmt.Sprintf("Patreon Slot #%d", i+1)
			slot, err := premium.CreatePremiumSlot(ctx, tx, v.DiscordID, premium.PremiumSourceTypePatreon, title, "Slot is available for as long as the pledge is active on patreon", int64(i+1), -1, premium.PremiumTierPremium)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "new CreatePremiumSlot")
			}

			logger.Info("Created new patreon premium slot #", slot.ID, v.DiscordID)
		}
		if slots > 0 {
			go premium.SendPremiumDM(v.DiscordID, premium.PremiumSourceTypePatreon, slots)
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
