package common

import (
	"regexp"

	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
)

type InviteSource struct {
	Name  string
	Regex *regexp.Regexp
}

var DiscordInviteSource = &InviteSource{
	Name:  "Discord",
	Regex: regexp.MustCompile(`(?i)(discord\.gg|discordapp\.com[\/\\]+invite|discord\.com[\/\\]+invite)(?:\/+#)?[\/\\]+([a-zA-Z0-9-]+)`),
}

var ThirdpartyDiscordSites = []*InviteSource{
	{Name: "discord.me", Regex: regexp.MustCompile(`(?i)discord\.me\/+.+`)},
	{Name: "invite.gg", Regex: regexp.MustCompile(`(?i)invite\.gg\/+.+`)},
	{Name: "discord.io", Regex: regexp.MustCompile(`(?i)discord\.io\/+.+`)},
	{Name: "discord.li", Regex: regexp.MustCompile(`(?i)discord\.li\/+.+`)},
	{Name: "disboard.org", Regex: regexp.MustCompile(`(?i)disboard\.org\/+server\/+join\/+.+`)},
	{Name: "discordy.com", Regex: regexp.MustCompile(`(?i)discordy\.com\/+server\+.php`)},
}

var AllInviteSources = append([]*InviteSource{DiscordInviteSource}, ThirdpartyDiscordSites...)

func ReplaceServerInvites(msg string, guildID int64, replacement string) string {

	for _, s := range AllInviteSources {
		msg = confusables.NormalizeQueryEncodedText(msg)
		msg = s.Regex.ReplaceAllString(msg, replacement)
	}

	return msg
}

func ContainsInvite(s string, checkDiscordSource, checkThirdPartySources bool) *InviteSource {
	s = confusables.NormalizeQueryEncodedText(s)
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
