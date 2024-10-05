package discordpremiumsource

import (
	"context"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
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
	go RunPoller()
}

func RunPoller() {
	ticker := time.NewTicker(time.Minute)
	for {
		<-ticker.C
		err := UpdatePremiumSlots(context.Background())
		if err != nil {
			logger.WithError(err).Error("Failed updating premium slots for discord")
		}
	}
}

func UpdatePremiumSlots(ctx context.Context) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	slots, err := models.PremiumSlots(qm.Where("source='discord'"), qm.OrderBy("id desc")).All(ctx, tx)
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
			// Need to create more slots
			for i := 0; i < slotsForPledge-len(userSlots); i++ {
				title := fmt.Sprintf("Discord Slot #%d", 1+i+len(userSlots))
				slot, err := premium.CreatePremiumSlot(ctx, tx, userID, "discord", title, "Slot is available as long as subscription is Active on Discord", int64(i+len(userSlots)), -1, premium.PremiumTierPremium)
				if err != nil {
					tx.Rollback()
					return errors.WithMessage(err, "CreatePremiumSlot")
				}
				logger.Info("Created discord premium slot #", slot.ID, slot.UserID)
			}
			go bot.SendDM(userID, fmt.Sprintf("You have received %d new premium slots via Discord Subscription, [Assign them to a server here](https://%s/premium)", slotsForPledge-len(userSlots), common.ConfHost.GetString()))
		} else if slotsForPledge < len(userSlots) {
			// Need to remove slots
			slotsToRemove := make([]int64, 0)
			for i := 0; i < len(userSlots)-slotsForPledge; i++ {
				slot := userSlots[i]
				slotsToRemove = append(slotsToRemove, slot.ID)
				logger.Info("Marked discord slot for deletion #", slot.ID, slot.UserID)
			}
			err = premium.RemovePremiumSlots(ctx, tx, userID, "discord", slotsToRemove)
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
			slot, err := premium.CreatePremiumSlot(ctx, tx, v.UserID, "discord", title, "Slot is available as long as subscription is Active on Discord", int64(i+1), -1, premium.PremiumTierPremium)
			if err != nil {
				tx.Rollback()
				return errors.WithMessage(err, "new CreatePremiumSlot")
			}
			logger.Info("Created new discord premium slot #", slot.ID, slot.ID)
		}
		go bot.SendDM(v.UserID, fmt.Sprintf("You have received %d new premium slots via Discord Subscription, [Assign them to a server here](https://%s/premium)", 1, common.ConfHost.GetString()))
	}
	err = tx.Commit()
	return errors.WithMessage(err, "Commit")
}
