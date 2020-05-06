package common

import (
	"regexp"
)

type InviteSource struct {
	Name  string
	Regex *regexp.Regexp
}

var DiscordInviteSource = &InviteSource{
	Name:  "Discord",
	Regex: regexp.MustCompile(`(?i)(discord\.gg|discordapp\.com\/invite|discord\.com\/invite)(?:\/#)?\/([a-zA-Z0-9-]+)`),
}

var ThirdpartyDiscordSites = []*InviteSource{
	&InviteSource{Name: "discord.me", Regex: regexp.MustCompile(`(?i)discord\.me\/.+`)},
	&InviteSource{Name: "invite.gg", Regex: regexp.MustCompile(`(?i)invite\.gg\/.+`)},
	&InviteSource{Name: "discord.io", Regex: regexp.MustCompile(`(?i)discord\.io\/.+`)},
	&InviteSource{Name: "discord.li", Regex: regexp.MustCompile(`(?i)discord\.li\/.+`)},
	&InviteSource{Name: "disboard.org", Regex: regexp.MustCompile(`(?i)disboard\.org\/server\/join\/.+`)},
	&InviteSource{Name: "discordy.com", Regex: regexp.MustCompile(`(?i)discordy\.com\/server\.php`)},

	// regexp.MustCompile(`disco\.gg\/.+`), Youc can't actually link to specific servers here can you, so not needed for now?
}

var AllInviteSources = append([]*InviteSource{DiscordInviteSource}, ThirdpartyDiscordSites...)

func ReplaceServerInvites(msg string, guildID int64, replacement string) string {

	for _, s := range AllInviteSources {
		msg = s.Regex.ReplaceAllString(msg, replacement)
	}

	return msg
}

func ContainsInvite(s string, checkDiscordSource, checkThirdPartySources bool) *InviteSource {
	for _, source := range AllInviteSources {
		if source == DiscordInviteSource && !checkDiscordSource {
			continue
		} else if source != DiscordInviteSource && !checkThirdPartySources {
			continue
		}

		if source.Regex.MatchString(s) {
			return source
		}
	}

	return nil
}
