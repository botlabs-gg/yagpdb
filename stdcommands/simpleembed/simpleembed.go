package simpleembed

import (
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"golang.org/x/image/colornames"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryTool,
	Name:                "SimpleEmbed",
	Aliases:             []string{"se"},
	Description:         "A more simpler version of CustomEmbed, controlled completely using switches.",
	RequireDiscordPerms: []int64{discordgo.PermissionManageMessages},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "channel", Help: "Optional channel to send in", Type: dcmd.Channel},
		{Name: "content", Help: "Text content for the message", Type: dcmd.String, Default: ""},

		{Name: "title", Type: dcmd.String, Default: ""},
		{Name: "desc", Type: dcmd.String, Help: "Text in the 'description' field", Default: ""},
		{Name: "color", Help: "Either hex code or name", Type: dcmd.String, Default: ""},
		{Name: "url", Help: "Url of this embed", Type: dcmd.String, Default: ""},
		{Name: "thumbnail", Help: "Url to a thumbnail", Type: dcmd.String, Default: ""},
		{Name: "image", Help: "Url to an image", Type: dcmd.String, Default: ""},

		{Name: "author", Help: "The text in the 'author' field", Type: dcmd.String, Default: ""},
		{Name: "authoricon", Help: "Url to a icon for the 'author' field", Type: dcmd.String, Default: ""},
		{Name: "authorurl", Help: "Url of the 'author' field", Type: dcmd.String, Default: ""},

		{Name: "footer", Help: "Text content for the footer", Type: dcmd.String, Default: ""},
		{Name: "footericon", Help: "Url to a icon for the 'footer' field", Type: dcmd.String, Default: ""},
	},
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		content := data.Switch("content").Str()
		embed := &discordgo.MessageEmbed{
			Title:       data.Switch("title").Str(),
			Description: data.Switch("desc").Str(),
			URL:         data.Switch("url").Str(),
		}

		if color := data.Switch("color").Str(); color != "" {
			parsedColor, ok := ParseColor(color)
			if !ok {
				return "Unknown color: " + color + ", can be either hex color code or name for a known color", nil
			}

			embed.Color = parsedColor
		}

		if author := data.Switch("author").Str(); author != "" {
			embed.Author = &discordgo.MessageEmbedAuthor{
				Name:    author,
				IconURL: data.Switch("authoricon").Str(),
				URL:     data.Switch("authorurl").Str(),
			}
		}

		if thumbnail := data.Switch("thumbnail").Str(); thumbnail != "" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: thumbnail,
			}
		}

		if image := data.Switch("image").Str(); image != "" {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: image,
			}
		}

		footer := data.Switch("footer").Str()
		footerIcon := data.Switch("footericon").Str()
		if footer != "" || footerIcon != "" {
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text:    footer,
				IconURL: footerIcon,
			}
		}

		cID := data.ChannelID
		c := data.Switch("channel")
		if c.Value != nil {
			cID = c.Value.(*dstate.ChannelState).ID

			hasPerms, err := bot.AdminOrPermMS(data.GuildData.GS.ID, cID, data.GuildData.MS, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages)
			if err != nil {
				return "Failed checking permissions, please try again or join the support server.", err
			}

			if !hasPerms {
				return "You do not have permissions to send messages there", nil
			}
		}

		if discordgo.IsEmbedEmpty(embed) {
			return "Cannot send an empty Embed", nil
		}

		messageSend := &discordgo.MessageSend{
			Content:         content,
			Embeds:          []*discordgo.MessageEmbed{embed},
			AllowedMentions: discordgo.AllowedMentions{},
		}
		_, err := common.BotSession.ChannelMessageSendComplex(cID, messageSend)
		if err != nil {
			return err, err
		}

		if cID != data.ChannelID {
			return "Done", nil
		}

		return nil, nil
	},
}

func ParseColor(raw string) (int, bool) {
	if strings.HasPrefix(raw, "#") {
		raw = raw[1:]
	}

	// try to parse as hex color code first
	parsed, err := strconv.ParseInt(raw, 16, 32)
	if err == nil {
		return int(parsed), true
	}

	// look up the color code table
	for _, v := range colornames.Names {
		if strings.EqualFold(v, raw) {
			cStruct := colornames.Map[v]

			color := (int(cStruct.R) << 16) | (int(cStruct.G) << 8) | int(cStruct.B)
			return color, true
		}
	}

	return 0, false
}
