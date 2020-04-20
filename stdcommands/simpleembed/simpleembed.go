package simpleembed

import (
	"strconv"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/image/colornames"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "SimpleEmbed",
	Aliases:     []string{"se"},
	Description: "A more simpler version of CustomEmbed, controlled completely using switches.",
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "channel", Help: "Optional channel to send in", Type: dcmd.Channel},

		&dcmd.ArgDef{Switch: "title", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "desc", Type: dcmd.String, Help: "Text in the 'description' field", Default: ""},
		&dcmd.ArgDef{Switch: "color", Help: "Either hex code or name", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "url", Help: "Url of this embed", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "thumbnail", Help: "Url to a thumbnail", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "image", Help: "Url to an image", Type: dcmd.String, Default: ""},

		&dcmd.ArgDef{Switch: "author", Help: "The text in the 'author' field", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "authoricon", Help: "Url to a icon for the 'author' field", Type: dcmd.String, Default: ""},

		&dcmd.ArgDef{Switch: "footer", Help: "Text content for the footer", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "footericon", Help: "Url to a icon for the 'footer' field", Type: dcmd.String, Default: ""},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
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

		cID := data.Msg.ChannelID
		c := data.Switch("channel")
		if c.Value != nil {
			cID = c.Value.(*dstate.ChannelState).ID

			hasPerms, err := bot.AdminOrPermMS(cID, data.MS, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages)
			if err != nil {
				return "Failed checking permissions, please try again or join the support server.", err
			}

			if !hasPerms {
				return "You do not have permissions to send messages there", nil
			}
		}

		_, err := common.BotSession.ChannelMessageSendEmbed(cID, embed)
		if err != nil {
			return err, err
		}

		if cID != data.Msg.ChannelID {
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
