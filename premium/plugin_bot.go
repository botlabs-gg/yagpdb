package premium

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	ccmodels "github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	rssmodels "github.com/botlabs-gg/yagpdb/v2/rss/models"
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
		return v2Message(premiumAccentActive,
			discordgo.TextDisplay{Content: "## YAGPDB Premium"},
			discordgo.TextDisplay{Content: "All servers are Premium on this instance — have fun!"},
		), nil
	}

	userID := data.Author.ID
	if common.IsOwner(data.Author.ID) && data.Switches["user"].Int() != 0 {
		userID = int64(data.Switches["user"].Int())
	}

	premiumSlots, err := models.PremiumSlots(qm.Where("user_id=?", userID)).CountG(data.Context())
	if err != nil {
		return "Failed fetching premium slots", err
	}

	slotsLine := fmt.Sprintf("<@%d> has **%d** premium slot(s).", userID, premiumSlots)

	if data.GuildData == nil {
		return v2Message(premiumAccentGold,
			discordgo.TextDisplay{Content: "## YAGPDB Premium"},
			discordgo.TextDisplay{Content: slotsLine + "\nUnlock higher limits and exclusive features on your servers."},
			discordgo.Separator{Divider: true},
			premiumActionsRow(discordStoreURL, patreonURL, perksURL, manageURL),
		), nil
	}

	guildID := data.GuildData.GS.ID
	isPremium, _ := IsGuildPremium(guildID)

	if isPremium {
		tier, _ := GuildPremiumTier(guildID)
		return v2Message(premiumAccentActive,
			discordgo.TextDisplay{Content: "## YAGPDB Premium · Active"},
			discordgo.TextDisplay{Content: fmt.Sprintf("This server has premium (tier: **%s**). Enjoy the perks!", tier.String())},
			discordgo.TextDisplay{Content: "-# " + slotsLine},
			discordgo.Separator{Divider: true},
			discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
				discordgo.Button{Style: discordgo.LinkButton, Label: "Manage Slots", URL: manageURL},
				discordgo.Button{Style: discordgo.LinkButton, Label: "Premium Perks", URL: perksURL},
			}},
		), nil
	}

	ccCount, _ := ccmodels.CustomCommands(qm.Where("guild_id = ?", guildID)).CountG(data.Context())
	rssCount, _ := rssmodels.RSSFeedSubscriptions(qm.Where("guild_id = ?", guildID)).CountG(data.Context())

	usage := fmt.Sprintf(
		"**Your current usage**\n"+
			"-# Custom commands · `%d / 100` *(premium: 250)*\n"+
			"-# RSS feeds · `%d / 2` *(premium: 10)*",
		ccCount, rssCount,
	)
	exclusives := "**Premium exclusives**\n" +
		"-# Trigger CCs on message edit · Custom Twitch announcements · Retroactive autorole scan · Threaded tickets · Custom bot avatar & banner · Bulk role assignment · Custom reputation regex"

	return v2Message(premiumAccentGold,
		discordgo.TextDisplay{Content: "## YAGPDB Premium"},
		discordgo.TextDisplay{Content: "Premium is not active on this server. Upgrade to unlock higher limits and exclusive features."},
		discordgo.Separator{Divider: true},
		discordgo.TextDisplay{Content: usage},
		discordgo.TextDisplay{Content: exclusives},
		discordgo.TextDisplay{Content: "-# " + slotsLine},
		discordgo.Separator{Divider: true},
		premiumActionsRow(discordStoreURL, patreonURL, perksURL, manageURL),
	), nil
}

func premiumActionsRow(discordURL, patreonURL, perksURL, manageURL string) discordgo.ActionsRow {
	return discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
		discordgo.Button{Style: discordgo.LinkButton, Label: "Get on Discord", URL: discordURL},
		discordgo.Button{Style: discordgo.LinkButton, Label: "Get on Patreon", URL: patreonURL},
		discordgo.Button{Style: discordgo.LinkButton, Label: "See All Perks", URL: perksURL},
		discordgo.Button{Style: discordgo.LinkButton, Label: "Manage Slots", URL: manageURL},
	}}
}

func v2Message(accentColor int, components ...discordgo.TopLevelComponent) *discordgo.MessageSend {
	container := discordgo.Container{AccentColor: accentColor}
	container.Components = append(container.Components, components...)
	return &discordgo.MessageSend{
		Components: []discordgo.TopLevelComponent{container},
		Flags:      discordgo.MessageFlagsIsComponentsV2,
	}
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
