package discordpremiumsource

import (
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func init() {
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleEntitlementCreate), eventsystem.EventEntitlementCreate)
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleEntitlementUpdate), eventsystem.EventEntitlementUpdate)
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleEntitlementDelete), eventsystem.EventEntitlementDelete)
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, CmdActivateTestEntitlement, CmdDeleteTestEntitlement)
}

func HandleEntitlementCreate(evt *eventsystem.EventData) {
	entitlement := evt.EntitlementCreate()
	logger.Infof("EntitlementCreate Event Received for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	if entitlement.SKUID != int64(confDiscordPremiumSKUID.GetInt()) {
		logger.Errorf("EntitlementCreate recieved for unknown SKUID %d", entitlement.SKUID)
		return
	}
	ctx := evt.Context()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		tx.Rollback()
		return
	}
	slots, err := models.PremiumSlots(qm.Where("source='discord'"), qm.Where("user_id = ?", entitlement.UserID)).All(ctx, tx)
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
	_, err = premium.CreatePremiumSlot(ctx, tx, entitlement.UserID, "discord", "Discord Slot #1", "Slot is available as long as subscription is Active on Discord", 1, -1, premium.PremiumTierPremium)
	if err != nil {
		logger.WithError(err).Error("Failed creating PremiumSlot for EntitlementCreate Event")
		tx.Rollback()
		return
	}
	err = tx.Commit()
	go bot.SendDM(entitlement.UserID, fmt.Sprintf("You have received %d new premium slots via Discord Subscription, [Assign them to a server here](https://%s/premium)", 1, common.ConfHost.GetString()))
	if err != nil {
		logger.WithError(err).Error("Failed committing transaction for EntitlementCreate Event")
	}
}

func HandleEntitlementUpdate(evt *eventsystem.EventData) {
	entitlement := evt.EntitlementUpdate()
	logger.Infof("EntitlementUpdate Event Received for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	if entitlement.SKUID != int64(confDiscordPremiumSKUID.GetInt()) {
		logger.Errorf("EntitlementUpdate recieved for unknown SKUID %d", entitlement.SKUID)
		return
	}
	ctx := evt.Context()
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
	slots, err := models.PremiumSlots(qm.Where("source='discord'"), qm.Where("user_id = ?", entitlement.UserID)).All(ctx, tx)
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

	err = premium.RemovePremiumSlots(ctx, tx, entitlement.UserID, "discord", []int64{slots[0].ID})
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

func HandleEntitlementDelete(evt *eventsystem.EventData) {
	entitlement := evt.EntitlementDelete()
	logger.Infof("EntitlementDelete Event Received for User %d and SKUID %d", entitlement.UserID, entitlement.SKUID)
	if entitlement.SKUID != int64(confDiscordPremiumSKUID.GetInt()) {
		logger.Errorf("EntitlementDelete recieved for unknown SKUID %d", entitlement.SKUID)
		return
	}
	ctx := evt.Context()
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(errors.WithMessage(err, "BeginTX"))
		tx.Rollback()
		return
	}

	slots, err := models.PremiumSlots(qm.Where("source='discord'"), qm.Where("user_id = ?", entitlement.UserID)).All(ctx, tx)
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

	err = premium.RemovePremiumSlots(ctx, tx, entitlement.UserID, "discord", []int64{slots[0].ID})
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

var CmdActivateTestEntitlement = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "activateTestEntitlement",
	Description:          "Enables Test Entitlement for a User. Bot Owner Only",
	HideFromHelp:         true,
	RequiredArgs:         3,
	RunInDM:              true,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.UserID},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		userID := data.Args[0].Int64()
		entitlementData := &discordgo.EntitlementTest{
			SKUID:     int64(confDiscordPremiumSKUID.GetInt()),
			OwnerID:   userID,
			OwnerType: discordgo.EntitlementOwnerTypeUserSubscription,
		}
		err := common.BotSession.EntitlementTestCreate(common.BotApplication.ID, entitlementData)
		if err != nil {
			return fmt.Sprintf("Failed enabling Entitlement for <@%d>: %s", userID, err), nil
		}
		return fmt.Sprintf("Enabled Entitlement for <@%d>", userID), nil
	}),
}

var CmdDeleteTestEntitlement = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "deleteTestEntitlement",
	Description:          "Delete Test Entitlement for a User. Bot Owner Only",
	HideFromHelp:         true,
	RequiredArgs:         3,
	RunInDM:              true,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.UserID},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		userID := data.Args[0].Int64()
		entitlements, err := common.BotSession.Entitlements(common.BotApplication.ID, &discordgo.EntitlementFilterOptions{
			UserID:       userID,
			SkuIDs:       []int64{int64(confDiscordPremiumSKUID.GetInt())},
			ExcludeEnded: true,
		})
		if err != nil {
			return fmt.Sprintf("Failed fetching Entitlements for <@%d>: %s", userID, err), nil
		}
		if len(entitlements) < 1 {
			return fmt.Sprintf("No Entitlements found for <@%d>", userID), nil
		}
		for _, v := range entitlements {
			//if v.Type == discordgo.EntitlementTypeTestModePurchase {
			err := common.BotSession.EntitlementTestDelete(common.BotApplication.ID, v.ID)
			if err != nil {
				return fmt.Sprintf("Failed deleting Entitlement for <@%d>: %s", userID, err), nil
			}
			//}
		}
		return fmt.Sprintf("Deleted Entitlement for <@%d>", userID), nil
	}),
}
