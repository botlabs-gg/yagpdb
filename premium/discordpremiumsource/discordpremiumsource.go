package discordpremiumsource

import (
	"context"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type DiscordPremiumSource struct{}

var ActiveDiscordPremiumPoller *DiscordPremiumPoller

func RegisterPlugin() {
	logger.Info("Registered discord premium source")
	premium.RegisterPremiumSource(&DiscordPremiumSource{})
	common.RegisterPlugin(&Plugin{})
}

func (ps *DiscordPremiumSource) Init() {}

func (ps *DiscordPremiumSource) Names() (human string, idname string) {
	return "Discord", "discord"
}

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Discord premium source",
		SysName:  "discord_premium_source",
		Category: common.PluginCategoryCore,
	}
}

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	if confDiscordPremiumSKUID.GetInt() == 0 {
		logger.Warn("No discord premium SKU ID set, not starting poller")
		return
	}
	ActiveDiscordPremiumPoller = InitPoller()
	if ActiveDiscordPremiumPoller == nil {
		logger.Warn("Failed initializing discord premium poller")
		return
	}

	pubsub.AddHandler("entitlement_create", handleEntitlementCreate, discordgo.EntitlementCreate{})
	pubsub.AddHandler("entitlement_update", handleEntitlementUpdate, discordgo.EntitlementUpdate{})
	pubsub.AddHandler("entitlement_delete", handleEntitlementDelete, discordgo.EntitlementDelete{})
	go RunPoller()
}

func RunPoller() {
	ticker := time.NewTicker(time.Minute * 10)
	for {
		<-ticker.C
		if !ActiveDiscordPremiumPoller.IsLastFetchSuccess() {
			logger.Warn("Last fetch was not successful, skipping update")
			continue
		}
		err := UpdatePremiumSlots(context.Background())
		if err != nil {
			logger.WithError(err).Error("Failed updating premium slots for discord")
		}
	}
}

func handleEntitlementCreate(evt *pubsub.Event) {
	entitlement := evt.Data.(*discordgo.EntitlementCreate)
	logger.Infof("EntitlementCreate Event Recieved for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	if entitlement.SKUID != int64(confDiscordPremiumSKUID.GetInt()) {
		logger.Errorf("EntitlementCreate recieved for unknown SKUID %d", entitlement.SKUID)
		return
	}
	ActiveDiscordPremiumPoller.activeEntitlements = append(ActiveDiscordPremiumPoller.activeEntitlements, entitlement.Entitlement)
	ctx := context.Background()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		tx.Rollback()
		return
	}
	slots, err := models.PremiumSlots(qm.Where("source=?", string(premium.PremiumSourceTypeDiscord)), qm.Where("user_id = ?", entitlement.UserID)).All(ctx, tx)
	if err != nil {
		logger.Error(errors.WithMessage(err, "Failed fetching PremiumSlots for EntitlementCreate"))
		tx.Rollback()
		return
	}
	if len(slots) > 0 {
		logger.Infof("User %d already has PremiumSlots", entitlement.UserID)
		tx.Rollback()
		return
	}
	_, err = premium.CreatePremiumSlot(ctx, tx, entitlement.UserID, premium.PremiumSourceTypeDiscord, "Discord Slot #1", "Slot is available as long as subscription is active on Discord", 1, -1, premium.PremiumTierPremium)
	if err != nil {
		logger.WithError(err).Error("Failed creating PremiumSlot for EntitlementCreate Event")
		tx.Rollback()
		return
	}
	err = tx.Commit()
	go premium.SendPremiumDM(entitlement.UserID, premium.PremiumSourceTypeDiscord, 1)
	if err != nil {
		logger.WithError(err).Error("Failed committing transaction for EntitlementCreate Event")
	}
}

func handleEntitlementDelete(evt *pubsub.Event) {
	entitlement := evt.Data.(*discordgo.EntitlementDelete)
	logger.Infof("EntitlementDelete Event Recieved for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	if entitlement.SKUID != int64(confDiscordPremiumSKUID.GetInt()) {
		logger.Errorf("EntitlementDelete recieved for unknown SKUID %d", entitlement.SKUID)
		return
	}
	ctx := context.Background()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		tx.Rollback()
		return
	}
	slots, err := models.PremiumSlots(qm.Where("source=?", string(premium.PremiumSourceTypeDiscord)), qm.Where("user_id = ?", entitlement.UserID)).All(ctx, tx)
	if err != nil {
		logger.Error(errors.WithMessage(err, "Failed fetching PremiumSlots for EntitlementDelete"))
		tx.Rollback()
		return
	}
	if len(slots) == 0 {
		logger.Infof("User %d has no PremiumSlots", entitlement.UserID)
		tx.Rollback()
		return
	}

	err = premium.RemovePremiumSlots(ctx, tx, entitlement.UserID, []int64{slots[0].ID})
	if err != nil {
		logger.WithError(err).Error("Failed Removing PremiumSlot for EntitlementDelete Event")
		tx.Rollback()
		return
	}
	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("Failed committing transaction for EntitlementDelete Event")
	}
}

func handleEntitlementUpdate(evt *pubsub.Event) {
	entitlement := evt.Data.(*discordgo.EntitlementUpdate)
	logger.Infof("EntitlementUpdate Event Recieved for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	if entitlement.SKUID != int64(confDiscordPremiumSKUID.GetInt()) {
		logger.Errorf("EntitlementUpdate recieved for unknown SKUID %d", entitlement.SKUID)
		return
	}
	ctx := context.Background()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		tx.Rollback()
		return
	}
	if entitlement.EndsAt != nil && entitlement.EndsAt.Before(time.Now()) {
		tx.Rollback()
		return
	}
	slots, err := models.PremiumSlots(qm.Where("source=?", string(premium.PremiumSourceTypeDiscord)), qm.Where("user_id = ?", entitlement.UserID)).All(ctx, tx)
	if err != nil {
		logger.Error(errors.WithMessage(err, "Failed fetching PremiumSlots for EntitlementUpdate"))
		tx.Rollback()
		return
	}
	if len(slots) == 0 {
		logger.Infof("User %d has no PremiumSlots", entitlement.UserID)
		tx.Rollback()
		return
	}

	err = premium.RemovePremiumSlots(ctx, tx, entitlement.UserID, []int64{slots[0].ID})
	if err != nil {
		logger.WithError(err).Error("Failed Removing PremiumSlot for EntitlementUpdate Event")
		tx.Rollback()
		return
	}
	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("Failed committing transaction for EntitlementUpdate Event")
	}
}

func UpdatePremiumSlots(ctx context.Context) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	slots, err := models.PremiumSlots(qm.Where("source=?", string(premium.PremiumSourceTypeDiscord)), qm.OrderBy("id desc")).All(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "PremiumSlots")
	}

	entitlements := ActiveDiscordPremiumPoller.GetEntitlements()
	if len(entitlements) == 0 {
		tx.Rollback()
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
		for _, entitlement := range entitlements {
			if entitlement.UserID == userID {
				//TODO: Add Support for multiple slots via discord
				slotsForPledge = 1
				break
			}
		}

		if slotsForPledge == len(userSlots) {
			// Correct number of slots already set up
			continue
		}

		if slotsForPledge > len(userSlots) {
			totalSlots := slotsForPledge - len(userSlots)
			// Need to create more slots
			for i := 0; i < totalSlots; i++ {
				title := fmt.Sprintf("Discord Slot #%d", 1+i+len(userSlots))
				slot, err := premium.CreatePremiumSlot(ctx, tx, userID, premium.PremiumSourceTypeDiscord, title, "Slot is available as long as subscription is Active on Discord", int64(i+len(userSlots)), -1, premium.PremiumTierPremium)
				if err != nil {
					tx.Rollback()
					return errors.WithMessage(err, "CreatePremiumSlot")
				}
				logger.Info("Created discord premium slot #", slot.ID, slot.UserID)
			}
			if totalSlots > 0 {
				go premium.SendPremiumDM(userID, premium.PremiumSourceTypeDiscord, totalSlots)
			}
		} else if slotsForPledge < len(userSlots) {
			// Need to remove slots
			slotsToRemove := make([]int64, 0)
			for i := 0; i < len(userSlots)-slotsForPledge; i++ {
				slot := userSlots[i]
				slotsToRemove = append(slotsToRemove, slot.ID)
				logger.Info("Marked discord slot for deletion #", slot.ID, slot.UserID)
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
	for _, v := range entitlements {
		for userID := range sorted {
			if userID == v.UserID {
				continue OUTER
			}
		}

		// If we got here then that means this is a new user
		slots := 1
		for i := 0; i < slots; i++ {
			title := fmt.Sprintf("Discord Premium Slot #%d", i+1)
			slot, err := premium.CreatePremiumSlot(ctx, tx, v.UserID, premium.PremiumSourceTypeDiscord, title, "Slot is available as long as subscription is active on Discord", int64(i+1), -1, premium.PremiumTierPremium)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "new CreatePremiumSlot")
			}
			logger.Info("Created new discord premium slot #", slot.ID, slot.ID)
		}
		if slots > 0 {
			go premium.SendPremiumDM(v.UserID, premium.PremiumSourceTypeDiscord, 1)
		}
	}
	err = tx.Commit()
	return errors.WithMessage(err, "Commit")
}
