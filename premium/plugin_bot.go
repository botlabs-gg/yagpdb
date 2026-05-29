package premium

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func init() {
	oldF := bot.StateLimitsF
	bot.StateLimitsF = func(guildID int64) (int, time.Duration) {
		premium, err := IsGuildPremium(guildID)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Failed checking if guild is premium")
			return oldF(guildID)
		}

		if premium {
			return PremiumStateMaxMessags, PremiumStateMaxMessageAge
		}

		return oldF(guildID)
	}
}

var cmdPremium = &commands.YAGCommand{
	CmdCategory:         commands.CategoryGeneral,
	Name:                "premium",
	Aliases:             []string{"premiumstatus", "premiumcheck", "perks"},
	Description:         "Shows YAGPDB premium status for this server and your premium slots.",
	RequiredArgs:        0,
	RunInDM:             true,
	SlashCommandEnabled: true,
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "user", Type: dcmd.UserID, Help: "Optional User to check premium slots for, Owner only", Default: 0},
	},
	RunFunc: runPremium,
}

const (
	premiumAccentActive = 0x2ecc71
	premiumAccentGold   = 0xf1c40f
)

func runPremium(data *dcmd.Data) (interface{}, error) {
	perksURL := fmt.Sprintf("%s/premium-perks", web.BaseURL())
	manageURL := fmt.Sprintf("%s/premium", web.BaseURL())
	discordStoreURL := fmt.Sprintf("https://discord.com/application-directory/%s/store", common.ConfClientID.GetString())
	patreonURL := "https://patreon.com/yagpdb/membership"

	if confAllGuildsPremium.GetBool() {
		return &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       "YAGPDB Premium",
				Description: "All servers are Premium on this instance, have fun!",
				Color:       premiumAccentActive,
			}},
			Flags: discordgo.MessageFlagsEphemeral,
		}, nil
	}

	userID := data.Author.ID
	if common.IsOwner(data.Author.ID) && data.Switches["user"].Int() != 0 {
		userID = int64(data.Switches["user"].Int())
	}

	premiumSlots, err := models.PremiumSlots(qm.Where("user_id=?", userID)).CountG(data.Context())
	if err != nil {
		return "Failed fetching premium slots", err
	}

	embed := &discordgo.MessageEmbed{Color: premiumAccentGold}
	if premiumSlots == 0 {
		embed.Title = "YAGPDB Premium"
		embed.Description = fmt.Sprintf("<@%d> doesn't have any premium slots. Unlock higher limits and exclusive features for your servers.", userID)
	} else {
		embed.Title = "YAGPDB Premium"
		embed.Color = premiumAccentActive
		embed.Description = fmt.Sprintf("<@%d> has **%d** premium slot(s).", userID, premiumSlots)
	}

	return &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.TopLevelComponent{
			premiumActionsRow(discordStoreURL, patreonURL, perksURL, manageURL),
		},
		Flags: discordgo.MessageFlagsEphemeral,
	}, nil
}

func premiumActionsRow(discordURL, patreonURL, perksURL, manageURL string) discordgo.ActionsRow {
	return discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
		discordgo.Button{Style: discordgo.LinkButton, Label: "Get on Discord", URL: discordURL},
		discordgo.Button{Style: discordgo.LinkButton, Label: "Get on Patreon", URL: patreonURL},
		discordgo.Button{Style: discordgo.LinkButton, Label: "Premium Perks", URL: perksURL},
		discordgo.Button{Style: discordgo.LinkButton, Label: "Manage Premium", URL: manageURL},
	}}
}

func (p *Plugin) BotInit() {
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmdGenerateCode, cmdPremium)
}

const (
	PremiumStateMaxMessags    = 10000
	PremiumStateMaxMessageAge = time.Hour * 12
)
