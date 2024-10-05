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

var cmdPremiumStatus = &commands.YAGCommand{
	CmdCategory:         commands.CategoryGeneral,
	Name:                "premiumstatus",
	Aliases:             []string{"gpc"},
	Description:         "Generates premium codes. Bot Owner Only",
	RequiredArgs:        0,
	RunInDM:             true,
	SlashCommandEnabled: true,
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.UserID, Help: "Optional User to check premium status for", Default: 0},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		userID := int64(data.Switches["User"].Int())
		if userID == 0 {
			userID = data.Author.ID
		}
		if confAllGuildsPremium.GetBool() {
			return "All guilds are premium, have fun!", nil
		}
		premiumSlots, err := models.PremiumSlots(qm.Where("user_id=?", userID)).AllG(data.Context())
		if err != nil {
			return "Failed Fetching Premium Slots ", err
		}
		embed := &discordgo.MessageEmbed{}
		if len(premiumSlots) < 1 {
			embed.Title = "No Premium Slots Found"
			embed.Description = fmt.Sprintf("<@%d> doesn have any premium slots! [Learn how to get premium](https://%s/premium-perks)", userID, common.ConfHost.GetString())
			return embed, nil
		}
		embed.Title = fmt.Sprintf("User has %d Premium Slots!", len(premiumSlots))
		embed.Description = fmt.Sprintf("<@%d> has %d premium slots! [Manage your premium slots](https://%s/premium)", userID, len(premiumSlots), common.ConfHost.GetString())
		return embed, nil
	},
}

func (p *Plugin) BotInit() {
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmdGenerateCode, cmdPremiumStatus)
}

const (
	PremiumStateMaxMessags    = 10000
	PremiumStateMaxMessageAge = time.Hour * 12
)
