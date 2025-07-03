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
	Aliases:             []string{"premiumcheck"},
	Description:         "Lets you check premium status of yourself or a specific user.",
	RequiredArgs:        0,
	RunInDM:             true,
	SlashCommandEnabled: false,
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "user", Type: dcmd.UserID, Help: "Optional User to check premium status for, Owner only", Default: 0},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var userID int64
		if common.IsOwner(data.Author.ID) && data.Switches["user"].Int() != 0 {
			userID = int64(data.Switches["user"].Int())
		} else {
			userID = data.Author.ID
		}

		if confAllGuildsPremium.GetBool() {
			return "All server are Premium, have fun!", nil
		}
		premiumSlots, err := models.PremiumSlots(qm.Where("user_id=?", userID)).CountG(data.Context())
		if err != nil {
			return "Failed Fetching Premium Slots ", err
		}
		embed := &discordgo.MessageEmbed{}
		if premiumSlots == 0 {
			embed.Title = "No Premium Slots Found"
			embed.Description = fmt.Sprintf("<@%d> does not have Premium!\n\n[Learn how to get Premium.](https://%s/premium-perks)", userID, common.ConfHost.GetString())
			return embed, nil
		}
		embed.Title = fmt.Sprintf("User has %d Premium Slot(s)!", premiumSlots)
		embed.Description = fmt.Sprintf("<@%d> has %d Premium Slot(s)!\n\n[Assign them to a server here.](https://%s/premium)", userID, premiumSlots, common.ConfHost.GetString())
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
