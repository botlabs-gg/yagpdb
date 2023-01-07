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
	Description:         "A more simpler version of CustomEmbed, controlled completely using switches.\nYou can edit existing messages by supplying the `-message` flag.",
	RequireDiscordPerms: []int64{discordgo.PermissionManageMessages},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "channel", Help: "Optional channel to send in", Type: dcmd.Channel},
		{Name: "message", Help: "Optional message ID to edit", Type: dcmd.BigInt},
		{Name: "content", Help: "Text content for the message", Type: dcmd.String},

		{Name: "title", Type: dcmd.String, Default: ""},
		{Name: "desc", Type: dcmd.String, Help: "Text in the 'description' field"},
		{Name: "color", Help: "Either hex code or name", Type: dcmd.String},
		{Name: "url", Help: "Url of this embed", Type: dcmd.String},
		{Name: "thumbnail", Help: "Url to a thumbnail", Type: dcmd.String},
		{Name: "image", Help: "Url to an image", Type: dcmd.String},

		{Name: "author", Help: "The text in the 'author' field", Type: dcmd.String},
		{Name: "authoricon", Help: "Url to a icon for the 'author' field", Type: dcmd.String},
		{Name: "authorurl", Help: "Url of the 'author' field", Type: dcmd.String},

		{Name: "footer", Help: "Text content for the footer", Type: dcmd.String},
		{Name: "footericon", Help: "Url to a icon for the 'footer' field", Type: dcmd.String},
	},
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		// Helper function to return value of a switch + whether it was set.
		getSwitch := func(key string) (value interface{}, set bool) {
			value = data.Switch(key).Value
			set = value != nil
			return
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

		mID := data.Switch("message").Int64()
		var msg *discordgo.Message
		if mID != 0 {
			hasPerms, err := bot.AdminOrPermMS(data.GuildData.GS.ID, cID, data.GuildData.MS, discordgo.PermissionManageMessages)
			if err != nil {
				return "Failed checking permissions, please try again or join the support server", err
			}

			if !hasPerms {
				return "You need the `Manage Messages` permission to be able to edit messages", nil
			}

			msg, err = common.BotSession.ChannelMessage(cID, mID)
			if err != nil || msg == nil {
				return "Failed fetching message to edit, check your channel and message IDs", nil
			}

			if msg.Author.ID != common.BotUser.ID {
				return "I can only edit messages sent by me", nil
			}
		}

		var embed *discordgo.MessageEmbed
		if msg != nil && len(msg.Embeds) > 0 {
			embed = msg.Embeds[0]
		} else {
			embed = &discordgo.MessageEmbed{}
		}

		modifiedEmbed := false

		if title, set := getSwitch("title"); set {
			embed.Title = title.(string)
			modifiedEmbed = true
		}
		if url, set := getSwitch("url"); set {
			embed.URL = url.(string)
			modifiedEmbed = true
		}
		if desc, set := getSwitch("desc"); set {
			embed.Description = desc.(string)
			modifiedEmbed = true
		}

		if color, set := getSwitch("color"); set {
			color := color.(string)
			if color == "" {
				// empty string resets the color
				embed.Color = 0
			} else {
				parsedColor, ok := ParseColor(color)
				if !ok {
					return "Unknown color: " + color + ", can be either hex color code or name for a known color", nil
				}
				embed.Color = parsedColor
			}

			modifiedEmbed = true
		}

		if embed.Author == nil {
			embed.Author = &discordgo.MessageEmbedAuthor{}
		}
		if name, set := getSwitch("author"); set {
			embed.Author.Name = name.(string)
			modifiedEmbed = true
		}
		if icon, set := getSwitch("authoricon"); set {
			embed.Author.IconURL = icon.(string)
			modifiedEmbed = true
		}
		if url, set := getSwitch("authorurl"); set {
			embed.Author.URL = url.(string)
			modifiedEmbed = true
		}

		if thumbnail, set := getSwitch("thumbnail"); set {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: thumbnail.(string)}
			modifiedEmbed = true
		}
		if image, set := getSwitch("image"); set {
			embed.Image = &discordgo.MessageEmbedImage{URL: image.(string)}
			modifiedEmbed = true
		}

		if embed.Footer == nil {
			embed.Footer = &discordgo.MessageEmbedFooter{}
		}
		if text, set := getSwitch("footer"); set {
			embed.Footer.Text = text.(string)
			modifiedEmbed = true
		}
		if icon, set := getSwitch("footericon"); set {
			embed.Footer.IconURL = icon.(string)
			modifiedEmbed = true
		}

		if msg == nil {
			send := &discordgo.MessageSend{AllowedMentions: discordgo.AllowedMentions{}}

			if modifiedEmbed {
				send.Embeds = []*discordgo.MessageEmbed{embed}
			}
			if content := data.Switch("content").Str(); content != "" {
				send.Content = content
			}

			if send.Content == "" && len(send.Embeds) == 0 {
				return "Cannot send an empty message", nil
			}

			_, err := common.BotSession.ChannelMessageSendComplex(cID, send)
			if err != nil {
				return err, err
			}
			if cID != data.ChannelID {
				return "Done", nil
			}
			return nil, nil
		}

		edit := &discordgo.MessageEdit{
			Content:         &msg.Content,
			AllowedMentions: discordgo.AllowedMentions{},
			ID:              mID,
			Channel:         cID,
		}

		if content, set := getSwitch("content"); set {
			v := content.(string)
			if v == "" {
				edit.Content = nil
			} else {
				edit.Content = &v
			}
		}
		if modifiedEmbed || (msg != nil && len(msg.Embeds) > 0) {
			edit.Embeds = []*discordgo.MessageEmbed{embed}
		}

		if edit.Content == nil && len(edit.Embeds) == 0 {
			return "Cannot edit a message to have no content and no embed", nil
		}

		_, err := common.BotSession.ChannelMessageEditComplex(edit)
		if err != nil {
			return err, err
		}
		return "Done", nil
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
