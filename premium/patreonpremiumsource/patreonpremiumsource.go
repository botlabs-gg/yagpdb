package patreonpremiumsource

import (
	"context"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/patreon"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
		updatePatreonPremiumSlots()
	}
}

func updatePatreonPremiumSlots() {
	oldPatreonUsers, err := models.PremiumSlots(
		qm.Select("DISTINCT user_id"),
		qm.Where("source=?", string(premium.PremiumSourceTypePatreon)),
	).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed fetching old entitled users from db")
		return
	}
	newPatronUsers := make(map[int64]*patreon.Patron)
	for _, v := range patreon.ActivePoller.GetPatrons() {
		newPatronUsers[v.DiscordID] = v
	}

	logger.Infof("Recalculating slots for %d new patreon users", len(newPatronUsers))
	for userID, patronage := range newPatronUsers {
		err := recalculatePatreonSlotsForUser(userID, patronage)
		if err != nil {
			logger.WithError(err).Errorf("Failed recalculating patreon slots for user %d", userID)
		}
	}

	logger.Infof("Recalculating slots for %d old entitled users", len(oldPatreonUsers))
	for _, row := range oldPatreonUsers {
		userID := row.UserID
		if _, ok := newPatronUsers[userID]; ok {
			continue
		}
		err := recalculatePatreonSlotsForUser(userID, nil)
		if err != nil {
			logger.WithError(err).Errorf("Failed recalculating patreon slots for user %d", userID)
		}
	}
}

func recalculatePatreonSlotsForUser(userID int64, patron *patreon.Patron) error {
	totalEntitledSlots := 0
	if patron != nil {
		totalEntitledSlots = fetchSlotForPatron(patron)
	}

	ctx := context.Background()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		return err
	}

	slots, err := models.PremiumSlots(qm.Where("source=? and deletes_at IS NULL", string(premium.PremiumSourceTypePatreon)), qm.Where("user_id = ?", userID)).All(ctx, tx)
	if err != nil {
		logger.Error(errors.WithMessage(err, "Failed fetching PremiumSlots for recalculatePatreonSlotsForUser"))
		tx.Rollback()
		return err
	}

	diff := totalEntitledSlots - len(slots)
	if diff != 0 {
		logger.Infof("Total entitled slots for user %d: %d, total existing slots: %d, diff: %d", userID, totalEntitledSlots, len(slots), diff)
	}
	if diff > 0 {
		markedForDeletion, err := premium.UserPremiumMarkedDeletedSlots(ctx, tx, userID, diff, premium.PremiumSourceTypePatreon)
		if err != nil {
			tx.Rollback()
			return err
		}
		if len(markedForDeletion) > 0 {
			logger.Infof("cancelling deletion for %d patreon slots for user %d", len(markedForDeletion), userID)
			err := premium.CancelSlotDeletionForUser(ctx, tx, userID, markedForDeletion)
			if err != nil {
				tx.Rollback()
				return err
			}
			diff -= len(markedForDeletion)
		}
		// Need to create more slots
		for i := 0; i < diff; i++ {
			nextID := len(slots) + len(markedForDeletion) + i + 1
			title := fmt.Sprintf("Patreon Slot #%d", nextID)
			slot, err := premium.CreatePremiumSlot(ctx, tx, userID, premium.PremiumSourceTypePatreon, title, "Slot is available for as long as the pledge is active on patreon", int64(nextID), -1, premium.PremiumTierPremium)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "CreatePremiumSlot")
			}
			logger.Info("Created patreon premium slot #", slot.ID, slot.UserID)
		}
		if diff > 0 {
			go premium.SendPremiumDM(userID, premium.PremiumSourceTypePatreon, diff)
		}
	} else if diff < 0 {
		slotsToRemove := make([]int64, 0)
		for i := 0; i < -diff; i++ {
			slot := slots[i]
			slotsToRemove = append(slotsToRemove, slot.ID)
			logger.Info("Marked patreon slot for deletion #", slot.ID, slot.UserID)
		}

		err = premium.MarkSlotsForDeletion(ctx, tx, userID, slotsToRemove)
		if err != nil {
			tx.Rollback()
			return errors.WithMessage(err, "MarkSlotsForDeletion")
		}
	}
	err = premium.RemoveMarkedDeletedSlotsForUser(ctx, tx, userID, premium.PremiumSourceTypePatreon)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "RemoveMarkedDeletedSlotsForUser")
	}

	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("Failed committing transaction for recalculatePatreonSlotsForUser")
		return errors.WithMessage(err, "Commit")
	}
	return nil
}

func fetchSlotForPatron(patron *patreon.Patron) (slots int) {
	if len(patron.Tiers) == 0 {
		return 0
	}

	tiers, err := models.PatreonTiers(models.PatreonTierWhere.TierID.IN(patron.Tiers)).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Errorf("Failed to fetch patreon tiers for tier_ids: %v", patron.Tiers)
		return 0
	}

	slots = 0
	for _, tier := range tiers {
		slots += tier.Slots
	}
	return slots
}
