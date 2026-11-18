package serverpreview

import (
	"fmt"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix/v3"
)

const (
	// MaxDescriptionLength matches the limit Discord's app puts on server descriptions.
	MaxDescriptionLength = 120

	defaultAccentColor = 0x5865F2
	previewCacheTTL    = 5 * time.Minute
)

var (
	builtPreviews = ccache.New(ccache.Configure().MaxSize(10000))
	guildCounts   = ccache.New(ccache.Configure().MaxSize(10000))
)

func getGuildWithCounts(guildID int64) (*discordgo.Guild, error) {
	item, err := guildCounts.Fetch(fmt.Sprint(guildID), previewCacheTTL, func() (any, error) {
		return common.BotSession.GuildWithCounts(guildID)
	})
	if err != nil {
		return nil, err
	}
	return item.Value().(*discordgo.Guild), nil
}

type Preview struct {
	GuildID     int64
	Name        string
	Description string
	IconURL     string
	BannerURL   string
	InviteURL   string
	Path        string
	EmbedJSON   []byte

	ShowMemberCount bool
	MemberCount     string
	OnlineCount     string
}

func newPreview(settings *models.ServerPreview, guild *discordgo.Guild) *Preview {
	preview := &Preview{
		GuildID:         guild.ID,
		Name:            guild.Name,
		IconURL:         guildIconURL(guild),
		BannerURL:       guildBannerURL(guild),
		ShowMemberCount: !settings.HideMemberCount,
		MemberCount:     formatCount(guild.ApproximateMemberCount),
		OnlineCount:     formatCount(guild.ApproximatePresenceCount),
	}
	if !settings.HideDescription {
		preview.Description = common.CutStringShort(guild.Description, MaxDescriptionLength)
	}
	return preview
}

func formatCount(n int) string {
	digits := fmt.Sprint(n)
	var out strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(d)
	}
	return out.String()
}

// GetPreview returns nil when the key has no preview that can be shown.
func GetPreview(key string) (*Preview, error) {
	guildID, viaSlug, err := resolveGuildID(key)
	if err != nil || guildID == 0 {
		return nil, err
	}

	settings, err := GetSettings(guildID)
	if err != nil || !settings.Enabled || settings.Suspended {
		return nil, err
	}

	onGuild, err := common.BotIsOnGuild(guildID)
	if err != nil || !onGuild {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%d:%d:%t", guildID, settings.UpdatedAt.UnixNano(), viaSlug)
	item, err := builtPreviews.Fetch(cacheKey, previewCacheTTL, func() (any, error) {
		return buildPreview(settings, viaSlug)
	})
	if err != nil {
		return nil, err
	}

	preview, _ := item.Value().(*Preview)
	return preview, nil
}

func buildPreview(settings *models.ServerPreview, viaSlug bool) (*Preview, error) {
	var banned bool
	if err := common.RedisPool.Do(radix.FlatCmd(&banned, "SISMEMBER", "banned_servers", settings.GuildID)); err != nil {
		return nil, err
	}
	if banned {
		return (*Preview)(nil), nil
	}

	isPremium, err := premium.IsGuildPremium(settings.GuildID)
	if err != nil {
		return nil, err
	}
	if viaSlug && (!isPremium || !settings.Slug.Valid) {
		return (*Preview)(nil), nil
	}

	guild, err := getGuildWithCounts(settings.GuildID)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/s/%d", guild.ID)
	if isPremium && settings.Slug.Valid {
		path = "/s/" + settings.Slug.String
	}

	preview := newPreview(settings, guild)
	preview.InviteURL = inviteURL(settings, guild)
	preview.Path = path
	if preview.InviteURL == "" {
		return (*Preview)(nil), nil
	}

	preview.EmbedJSON, err = previewEmbed(settings, preview, isPremium).Encode()
	if err != nil {
		return nil, err
	}

	return preview, nil
}

func previewEmbed(settings *models.ServerPreview, preview *Preview, isPremium bool) web.ComponentEmbed {
	container := discordgo.Container{AccentColor: defaultAccentColor}
	if isPremium && settings.AccentColor != 0 {
		container.AccentColor = settings.AccentColor
	}

	if preview.BannerURL != "" {
		container.Components = append(container.Components, discordgo.MediaGallery{Items: []discordgo.MediaGalleryItem{
			{Media: discordgo.UnfurledMediaItem{URL: preview.BannerURL}},
		}})
	}

	header := []discordgo.SectionComponentPart{discordgo.TextDisplay{Content: "## " + escapeMarkdown(preview.Name)}}
	if preview.ShowMemberCount {
		header = append(header, discordgo.TextDisplay{Content: fmt.Sprintf("🟢 **%s** Online \u2003 ⚪ **%s** Members", preview.OnlineCount, preview.MemberCount)})
	}

	if preview.IconURL != "" {
		container.Components = append(container.Components, discordgo.Section{
			Components: header,
			Accessory:  discordgo.Thumbnail{Media: discordgo.UnfurledMediaItem{URL: preview.IconURL}},
		})
	} else {
		for _, part := range header {
			container.Components = append(container.Components, part.(discordgo.TextDisplay))
		}
	}

	if preview.Description != "" {
		container.Components = append(container.Components,
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
			discordgo.TextDisplay{Content: escapeMarkdown(preview.Description)},
		)
	}

	if isPremium && settings.CustomText != "" {
		container.Components = append(container.Components,
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
			discordgo.TextDisplay{Content: settings.CustomText},
		)
	}

	var buttons []discordgo.InteractiveComponent
	if preview.InviteURL != "" {
		buttons = append(buttons, web.LinkButton("Join Server", preview.InviteURL))
	}
	if isPremium {
		for _, b := range ParseButtons(settings.Buttons) {
			buttons = append(buttons, web.LinkButton(b.Label, b.URL))
		}
	}
	if len(buttons) > 0 {
		container.Components = append(container.Components,
			discordgo.Separator{Spacing: discordgo.SeparatorSpacingSmall},
			discordgo.ActionsRow{Components: buttons},
		)
	}

	if !isPremium || !settings.HideFooter {
		container.Components = append(container.Components, discordgo.TextDisplay{Content: fmt.Sprintf("-# Powered by [%s](%s)", escapeMarkdown(botName()), web.BaseURL())})
	}

	return web.ComponentEmbed{Component: container}
}

func botName() string {
	if common.BotUser == nil {
		return "YAGPDB"
	}
	if common.BotUser.Globalname != "" {
		return common.BotUser.Globalname
	}
	return common.BotUser.Username
}

func guildIconURL(guild *discordgo.Guild) string {
	if guild.Icon == "" {
		return ""
	}
	return discordgo.EndpointGuildIcon(guild.ID, guild.Icon) + "?size=256"
}

func guildBannerURL(guild *discordgo.Guild) string {
	if guild.Banner == "" {
		return ""
	}
	return discordgo.EndpointGuildBanner(guild.ID, guild.Banner) + "?size=1024"
}

func inviteURL(settings *models.ServerPreview, guild *discordgo.Guild) string {
	if guild.VanityURLCode != "" {
		return "https://discord.gg/" + guild.VanityURLCode
	}
	if settings.InviteCode == "" {
		return ""
	}
	return "https://discord.gg/" + settings.InviteCode
}

func CreateInvite(channelID int64) (string, error) {
	if channelID == 0 {
		return "", nil
	}

	invite, err := common.BotSession.ChannelInviteCreate(channelID, discordgo.Invite{MaxAge: 0, MaxUses: 0})
	if err != nil {
		return "", err
	}
	return invite.Code, nil
}

var markdownEscaper = strings.NewReplacer(
	`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `~`, `\~`, `|`, `\|`,
	`>`, `\>`, `#`, `\#`, `[`, `\[`, `]`, `\]`, `(`, `\(`, `)`, `\)`, `-`, `\-`,
)

func escapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}
