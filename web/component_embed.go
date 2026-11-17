package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// https://docs.discord.com/developers/link-previews/component-embeds
const (
	componentEmbedMaxBytes      = 3000
	componentEmbedMaxComponents = 40
	yagpdbAccentColor           = 0xE74C3C
)

var ErrComponentEmbedTooLarge = errors.New("component embed exceeds discord's size limits")

type ComponentEmbed struct {
	Component discordgo.Container `json:"component"`
}

func (c ComponentEmbed) Encode() ([]byte, error) {
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	if len(encoded) > componentEmbedMaxBytes || countComponents(c.Component) > componentEmbedMaxComponents {
		return nil, ErrComponentEmbedTooLarge
	}

	return encoded, nil
}

func countComponents(c discordgo.MessageComponent) int {
	switch t := c.(type) {
	case discordgo.Container:
		n := 1
		for _, child := range t.Components {
			n += countComponents(child)
		}
		return n
	case discordgo.Section:
		n := 1 + countComponents(t.Accessory)
		for _, child := range t.Components {
			n += countComponents(child)
		}
		return n
	case discordgo.ActionsRow:
		return 1 + len(t.Components)
	}
	return 1
}

func LinkButton(label, url string) discordgo.Button {
	return discordgo.Button{Label: label, Style: discordgo.LinkButton, URL: url}
}

func botInviteURL() string {
	return fmt.Sprintf("https://discord.com/oauth2/authorize?client_id=%s&scope=applications.commands%%20bot&permissions=%d",
		common.ConfClientID.GetString(), BotInvitePermissions)
}

const landingFeatureList = `### Features
- **Custom Commands** with a full templating language
- **Basic & Advanced Automoderator**
- **Moderation**: warnings, mutes, timeouts, kicks and bans with logging
- **Role Menus** and self-assignable roles
- **Feeds**: Reddit, YouTube, RSS, Twitch and Streaming announcements
- **Notifications**: join, leave and boost messages
- **Server Stats**, **Logging**, **Tickets**, **Reminders**, **Reputation**, **Verification** and more`

func landingComponentEmbed() ComponentEmbed {
	base := BaseURL()

	return ComponentEmbed{Component: discordgo.Container{
		AccentColor: yagpdbAccentColor,
		Components: []discordgo.TopLevelComponent{
			discordgo.Section{
				Components: []discordgo.SectionComponentPart{
					discordgo.TextDisplay{Content: fmt.Sprintf("# [YAGPDB](%s)\nThe most powerful and customizable discord bot ever created", base)},
				},
				Accessory: discordgo.Thumbnail{Media: discordgo.UnfurledMediaItem{URL: base + "/static/icons/android-chrome-512x512.png"}},
			},
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
			discordgo.TextDisplay{Content: landingFeatureList},
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
			discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
				LinkButton("Add to Server", botInviteURL()),
				LinkButton("Documentation", "https://help.yagpdb.xyz"),
				LinkButton("Premium", base+"/premium-perks"),
				LinkButton("GitHub", "https://github.com/botlabs-gg/yagpdb"),
			}},
		},
	}}
}

func HandleLandingComponentEmbed(w http.ResponseWriter, r *http.Request) {
	serveComponentEmbed(w, r, landingComponentEmbed())
}

func serveComponentEmbed(w http.ResponseWriter, r *http.Request, embed ComponentEmbed) {
	encoded, err := embed.Encode()
	if err != nil {
		CtxLogger(r.Context()).WithError(err).Error("failed encoding component embed")
		http.Error(w, "failed encoding component embed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.discord.component-embed+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(encoded)
}
