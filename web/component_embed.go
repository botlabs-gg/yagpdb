package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

type landingFeature struct {
	label   string
	sysName string
}

type landingFeatureLine struct {
	format   string
	features []landingFeature
}

var landingFeatureLines = []landingFeatureLine{
	{"%s with a full templating language", []landingFeature{{"**Custom Commands**", "custom_commands"}}},
	{"%s", []landingFeature{{"**Basic Automoderator**", "legacy_automod"}, {"**Advanced Automoderator**", "automod_v2"}}},
	{"%s: warnings, mutes, timeouts, kicks and bans with logging", []landingFeature{{"**Moderation**", "moderation"}}},
	{"%s and self-assignable roles", []landingFeature{{"**Role Menus**", "role_commands"}}},
	{"**Feeds**: %s", []landingFeature{{"Reddit", "reddit"}, {"YouTube", "youtube"}, {"RSS", "rss"}, {"Twitch", "twitch"}, {"Streaming announcements", "streaming"}}},
	{"%s: join, leave and boost messages", []landingFeature{{"**Notifications**", "notifications"}}},
	{"%s and more", []landingFeature{{"**Server Stats**", "server_stats"}, {"**Logging**", "logging"}, {"**Tickets**", "tickets"}, {"**Reminders**", "reminders"}, {"**Reputation**", "reputation"}, {"**Verification**", "verification"}}},
}

func landingFeatureList() string {
	var lines []string
	for _, line := range landingFeatureLines {
		var active []string
		for _, f := range line.features {
			if common.PluginActive(f.sysName) {
				active = append(active, f.label)
			}
		}
		if len(active) > 0 {
			lines = append(lines, "- "+fmt.Sprintf(line.format, joinWithAnd(active)))
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return "### Features\n" + strings.Join(lines, "\n")
}

func joinWithAnd(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func landingComponentEmbed() ComponentEmbed {
	base := BaseURL()

	embed := ComponentEmbed{Component: discordgo.Container{
		AccentColor: yagpdbAccentColor,
		Components: []discordgo.TopLevelComponent{
			discordgo.Section{
				Components: []discordgo.SectionComponentPart{
					discordgo.TextDisplay{Content: fmt.Sprintf("# [YAGPDB](%s)\nThe most powerful and customizable discord bot ever created", base)},
				},
				Accessory: discordgo.Thumbnail{Media: discordgo.UnfurledMediaItem{URL: base + "/static/icons/android-chrome-512x512.png"}},
			},
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
		},
	}}

	if features := landingFeatureList(); features != "" {
		embed.Component.Components = append(embed.Component.Components,
			discordgo.TextDisplay{Content: features},
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
		)
	}

	embed.Component.Components = append(embed.Component.Components, discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
		LinkButton("Add to Server", botInviteURL()),
		LinkButton("Documentation", "https://help.yagpdb.xyz"),
		LinkButton("Premium", base+"/premium-perks"),
		LinkButton("GitHub", "https://github.com/botlabs-gg/yagpdb"),
	}})

	return embed
}

var landingComponentEmbedJSON []byte

func loadLandingComponentEmbed() {
	encoded, err := landingComponentEmbed().Encode()
	if err != nil {
		logger.WithError(err).Error("failed encoding the landing page component embed")
		return
	}
	landingComponentEmbedJSON = encoded
}

func HandleLandingComponentEmbed(w http.ResponseWriter, r *http.Request) {
	if landingComponentEmbedJSON == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.discord.component-embed+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(landingComponentEmbedJSON)
}
