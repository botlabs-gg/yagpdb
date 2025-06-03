package moderation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type ModlogAction struct {
	Prefix string
	Emoji  string
	Color  int

	Footer string
}

func (m ModlogAction) String() string {
	str := m.Emoji + m.Prefix
	if m.Footer != "" {
		str += " (" + m.Footer + ")"
	}

	return str
}

var (
	MAMute           = ModlogAction{Prefix: "Muted", Emoji: "🔇", Color: 0x57728e}
	MAUnmute         = ModlogAction{Prefix: "Unmuted", Emoji: "🔊", Color: 0x62c65f}
	MAKick           = ModlogAction{Prefix: "Kicked", Emoji: "👢", Color: 0xf2a013}
	MABanned         = ModlogAction{Prefix: "Banned", Emoji: "🔨", Color: 0xd64848}
	MAUnbanned       = ModlogAction{Prefix: "Unbanned", Emoji: "🔓", Color: 0x62c65f}
	MAWarned         = ModlogAction{Prefix: "Warned", Emoji: "⚠", Color: 0xfca253}
	MADelwarn      	 = ModlogAction{Prefix: "Warning removed from", Emoji: "🧽", Color: 0xfca253}
	MAGiveRole       = ModlogAction{Prefix: "", Emoji: "➕", Color: 0x53fcf9}
	MARemoveRole     = ModlogAction{Prefix: "", Emoji: "➖", Color: 0x53fcf9}
	MAClearWarnings  = ModlogAction{Prefix: "Cleared warnings", Emoji: "👌", Color: 0x62c65f}
)

func CreateModlogEmbed(config *Config, author *discordgo.User, action ModlogAction, target *discordgo.User, reason, logLink string) error {
	channelID := config.ActionChannel
	if channelID == 0 {
		return nil
	}

	emptyAuthor := false
	if author == nil {
		emptyAuthor = true
		author = &discordgo.User{
			ID:            0,
			Username:      "Unknown",
			Discriminator: "????",
		}
	}

	if reason == "" {
		reason = "(no reason specified)"
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (ID %d)", author.String(), author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: discordgo.EndpointUserAvatar(target.ID, target.Avatar),
		},
		Color: action.Color,
		Description: fmt.Sprintf("**%s%s** %s *(ID %d)*\n📄**Reason:** %s",
			action.Emoji, action.Prefix, target.String(), target.ID, reason),
	}

	if logLink != "" {
		embed.Description += " ([Logs](" + logLink + "))"
	}

	if action.Footer != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: action.Footer,
		}
	}

	m, err := common.BotSession.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		logger.WithError(err).Errorf("Creating modlog embed for guild_id %d, action_channel %d", config.GuildID, config.ActionChannel)
		if common.IsDiscordErr(err, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownChannel) {
			// disable the modlog
			config.ActionChannel = 0
			SaveConfig(config)
			return nil
		}
		return err
	}

	if emptyAuthor {
		placeholder := fmt.Sprintf("Assign an author and reason to this using **`reason %d your-reason-here`**", m.ID)
		updateEmbedReason(nil, placeholder, embed)
		_, err = common.BotSession.ChannelMessageEditEmbed(channelID, m.ID, embed)
	}
	return err
}

var (
	logsRegex = regexp.MustCompile(`\(\[Logs\]\(.*\)\)`)
)

func updateEmbedReason(author *discordgo.User, reason string, embed *discordgo.MessageEmbed) {
	const checkStr = "📄**Reason:**"

	index := strings.Index(embed.Description, checkStr)
	withoutReason := embed.Description[:index+len(checkStr)]

	logsLink := logsRegex.FindString(embed.Description)
	if logsLink != "" {
		logsLink = " " + logsLink
	}

	embed.Description = withoutReason + " " + reason + logsLink

	if author != nil {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (ID %d)", author.String(), author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		}
	}
}
