package discordpremiumsource

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
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
	pubsub.Publish("entitlement_create", -1, entitlement)
}

func HandleEntitlementUpdate(evt *eventsystem.EventData) {
	entitlement := evt.EntitlementUpdate()
	pubsub.Publish("entitlement_update", -1, entitlement)
}

func HandleEntitlementDelete(evt *eventsystem.EventData) {
	entitlement := evt.EntitlementDelete()
	pubsub.Publish("entitlement_delete", -1, entitlement)
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
		{Name: "SKUID", Type: dcmd.Int},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		userID := data.Args[0].Int64()
		skuID := data.Args[1].Int64()
		slots := fetchSlotsForDiscordSku(int64(skuID))
		if slots == 0 {
			return fmt.Sprintf("No Slots configured for skuID %d", skuID), nil
		}

		entitlementData := &discordgo.EntitlementTest{
			SKUID:     skuID,
			OwnerID:   userID,
			OwnerType: discordgo.EntitlementOwnerTypeUserSubscription,
		}
		err := common.BotSession.EntitlementTestCreate(common.BotApplication.ID, entitlementData)
		if err != nil {
			return fmt.Sprintf("Failed enabling Entitlement for <@%d>: %s", userID, err), nil
		}
		return fmt.Sprintf("Enabled Entitlement with %d slots for <@%d>", slots, userID), nil
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
		{Name: "SKUID", Type: dcmd.Int},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		userID := data.Args[0].Int64()
		skuID := data.Args[1].Int64()
		slots := fetchSlotsForDiscordSku(int64(skuID))
		if slots == 0 {
			return fmt.Sprintf("No Slots configured for skuID %d", skuID), nil
		}
		entitlements, err := common.BotSession.Entitlements(common.BotApplication.ID, &discordgo.EntitlementFilterOptions{
			UserID:       userID,
			SkuIDs:       []int64{skuID},
			ExcludeEnded: true,
		})
		if err != nil {
			return fmt.Sprintf("Failed fetching Entitlements for <@%d>: %s", userID, err), nil
		}
		if len(entitlements) < 1 {
			return fmt.Sprintf("No Entitlements found for <@%d>", userID), nil
		}
		for _, v := range entitlements {
			err := common.BotSession.EntitlementTestDelete(common.BotApplication.ID, v.ID)
			if err != nil {
				return fmt.Sprintf("Failed deleting Entitlement for <@%d>: %s", userID, err), nil
			}
		}
		return fmt.Sprintf("Deleted Entitlement for <@%d>", userID), nil
	}),
}
