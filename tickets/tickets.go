package tickets

//go:generate sqlboiler --no-hooks psql

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/tickets/models"
	"github.com/sirupsen/logrus"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Tickets",
		SysName:  "tickets",
		Category: common.PluginCategoryMisc,
	}
}

func RegisterPlugin() {
	if !common.InitSchema(DBSchema, "tickets") {
		logrus.Error("[tickets] failed initializing tickets schema, not enabling plugin")
		return
	}

	common.RegisterPlugin(&Plugin{})
}

const (
	DefaultTicketMsg        = "{{$embed := cembed `description` (joinStr `` `Welcome ` .User.Mention `\n\nPlease describe the reasoning for opening this ticket, include any information you think may be relevant such as proof, other third parties and so on.` " + DefaultTicketMsgClose + DefaultTicketMsgAddUser + ")}}\n{{sendMessage nil $embed}}"
	DefaultTicketMsgClose   = "\n\"\\n\\nuse the following command to close the ticket\\n\"\n\"`-ticket close reason for closing here`\\n\\n\""
	DefaultTicketMsgAddUser = "\n\"use the following command to add users to the ticket\\n\"\n\"`-ticket adduser @user`\""
)

func TicketLog(conf *models.TicketConfig, guildID int64, author *discordgo.User, embed *discordgo.MessageEmbed) {
	if conf.StatusChannel == 0 {
		return
	}

	embed.Author = &discordgo.MessageEmbedAuthor{
		Name:    fmt.Sprintf("%s#%s (%d)", author.Username, author.Discriminator, author.ID),
		IconURL: author.AvatarURL("128"),
	}

	_, err := common.BotSession.ChannelMessageSendEmbed(conf.StatusChannel, embed)
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("[tickets] failed sending log message to guild")
	}
}
