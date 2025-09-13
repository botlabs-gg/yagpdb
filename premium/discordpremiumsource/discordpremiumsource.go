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

// Recalculate slots for a user by fetching entitlements from Discord REST API
func recalculateDiscordSlotsForUser(userID int64) error {
	entitlements, err := common.BotSession.Entitlements(common.BotApplication.ID, &discordgo.EntitlementFilterOptions{
		UserID:       userID,
		ExcludeEnded: true,
	})

	if err != nil {
		logger.WithError(err).Errorf("Failed to fetch entitlements for user %d from Discord", userID)
		return err
	}

	totalEntitledSlots := 0
	for _, e := range entitlements {
		totalEntitledSlots += fetchSlotsForDiscordSku(e.SKUID)
	}

	ctx := context.Background()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		return err
	}

	slots, err := models.PremiumSlots(qm.Where("source=?", string(premium.PremiumSourceTypeDiscord)), qm.Where("user_id = ?", userID)).All(ctx, tx)
	if err != nil {
		logger.Error(errors.WithMessage(err, "Failed fetching PremiumSlots for recalculateDiscordSlotsForUser"))
		tx.Rollback()
		return err
	}

	diff := totalEntitledSlots - len(slots)
	if diff == 0 {
		tx.Rollback()
		return nil
	}

	if diff > 0 {
		// Add slots
		for i := range diff {
			title := fmt.Sprintf("Discord Slot #%d", 1+i+len(slots))
			slot, err := premium.CreatePremiumSlot(ctx, tx, userID, premium.PremiumSourceTypeDiscord, title, "Slot is available as long as subscription is active on Discord", int64(i+len(slots)), -1, premium.PremiumTierPremium)
			if err != nil {
				logger.WithError(err).Error("Failed creating PremiumSlot for recalculateDiscordSlotsForUser")
				tx.Rollback()
				return err
			}
			logger.Info("Created discord premium slot #", slot.ID, slot.UserID)
		}
		if diff > 0 {
			go premium.SendPremiumDM(userID, premium.PremiumSourceTypeDiscord, diff)
		}
	} else if diff < 0 {
		// Remove slots
		slotsToRemove := make([]int64, 0)
		for i := 0; i < -diff; i++ {
			slot := slots[i]
			slotsToRemove = append(slotsToRemove, slot.ID)
			logger.Info("Marked discord slot for deletion #", slot.ID, slot.UserID)
		}
		err = premium.RemovePremiumSlots(ctx, tx, userID, slotsToRemove)
		if err != nil {
			logger.WithError(err).Error("Failed removing PremiumSlots for recalculateDiscordSlotsForUser")
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("Failed committing transaction for recalculateDiscordSlotsForUser")
		return err
	}
	return nil
}

func handleEntitlementCreate(evt *pubsub.Event) {
	entitlement := evt.Data.(*discordgo.EntitlementCreate)
	logger.Infof("EntitlementCreate Event Recieved for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	ActiveDiscordPremiumPoller.activeEntitlements = append(ActiveDiscordPremiumPoller.activeEntitlements, entitlement.Entitlement)
	recalculateDiscordSlotsForUser(entitlement.UserID)
}

func handleEntitlementDelete(evt *pubsub.Event) {
	entitlement := evt.Data.(*discordgo.EntitlementDelete)
	logger.Infof("EntitlementDelete Event Recieved for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	recalculateDiscordSlotsForUser(entitlement.UserID)
}

func handleEntitlementUpdate(evt *pubsub.Event) {
	entitlement := evt.Data.(*discordgo.EntitlementUpdate)
	logger.Infof("EntitlementUpdate Event Recieved for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	recalculateDiscordSlotsForUser(entitlement.UserID)
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
	existingUserSlots := make(map[int64][]*models.PremiumSlot)
	for _, slot := range slots {
		existingUserSlots[slot.UserID] = append(existingUserSlots[slot.UserID], slot)
	}

	// recalculate slots for each user
	for userID := range existingUserSlots {
		recalculateDiscordSlotsForUser(userID)
	}

	// Add completely new premium slots
	for _, v := range entitlements {
		if _, ok := existingUserSlots[v.UserID]; ok {
			continue
		}

		// If we got here then that means this is a new user
		slots := fetchSlotsForDiscordSku(v.SKUID)
		for i := range slots {
			title := fmt.Sprintf("Discord Slot #%d", i+1)
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

func fetchSlotsForDiscordSku(skuID int64) int {
	sku, err := models.DiscordSkus(models.DiscordSkuWhere.SkuID.EQ(skuID)).OneG(context.Background())
	if err != nil {
		logger.WithError(err).Errorf("Failed to fetch discord skus for sku_id: %d", skuID)
		return 0
	}
	return sku.Slots
}
