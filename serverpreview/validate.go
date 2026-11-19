package serverpreview

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/antiphishing"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/safebrowsing"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview/models"
)

const (
	// MaxPreviewBytes leaves headroom under Discord's 3000 byte limit, so a valid preview never has to be cut down.
	MaxPreviewBytes = 2500

	MaxCustomTextLength  = 500
	MaxButtons           = 3
	MaxButtonLabelLength = 40
	MaxButtonURLLength   = 512
)

var (
	slugRegex       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)
	maskedLinkRegex = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)

	reservedSlugParts = []string{"yagpdb", "discord", "botlabs", "nitro"}
	reservedSlugs     = map[string]bool{
		"admin": true, "administrator": true, "api": true, "help": true, "mod": true, "moderator": true,
		"official": true, "staff": true, "static": true, "support": true, "system": true, "verify": true,
	}
)

func ValidateSlug(slug string) error {
	if !slugRegex.MatchString(slug) {
		return errors.New("the slug must be 3 to 32 characters of lowercase letters, numbers and hyphens, and can't start or end with a hyphen")
	}

	if strings.Trim(slug, "0123456789") == "" {
		return errors.New("the slug can't be only numbers")
	}

	if reservedSlugs[slug] {
		return errors.New("that slug is reserved")
	}
	for _, part := range reservedSlugParts {
		if strings.Contains(slug, part) {
			return errors.New("the slug can't contain \"" + part + "\"")
		}
	}

	return nil
}

func ValidateCustomText(text string, guildID int64) error {
	if utf8.RuneCountInString(text) > MaxCustomTextLength {
		return errors.New("the custom text is longer than 500 characters")
	}

	if maskedLinkRegex.MatchString(text) {
		return errors.New("the custom text can't contain masked links like [text](url), paste the link directly instead")
	}

	return checkLinks(text, guildID)
}

func ValidateButtons(buttons []Button, guildID int64) error {
	if len(buttons) > MaxButtons {
		return errors.New("a preview can have at most 3 custom buttons")
	}

	for _, b := range buttons {
		if b.Label == "" || utf8.RuneCountInString(b.Label) > MaxButtonLabelLength {
			return errors.New("button labels must be 1 to 40 characters")
		}

		if len(b.URL) > MaxButtonURLLength {
			return errors.New("button links can be at most 512 characters")
		}

		parsed, err := url.Parse(b.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return errors.New("button links must be full https:// links")
		}

		if err := checkLinks(b.URL, guildID); err != nil {
			return err
		}
	}

	return nil
}

// ValidatePreviewSize checks the preview fits within MaxPreviewBytes for the largest server details Discord allows.
func ValidatePreviewSize(settings *models.ServerPreview) error {
	longest := strings.Repeat("\U0001F525", 100)
	guild := &discordgo.Guild{
		ID:                       math.MaxInt64,
		Name:                     longest,
		Description:              longest + longest,
		Icon:                     "a_" + strings.Repeat("0", 32),
		Banner:                   "a_" + strings.Repeat("0", 32),
		ApproximateMemberCount:   math.MaxInt32,
		ApproximatePresenceCount: math.MaxInt32,
	}
	worstCase := *settings
	worstCase.HideFooter = false
	worstCase.HideMemberCount = false
	worstCase.HideDescription = false
	worstCase.HideBanner = false

	preview := newPreview(&worstCase, guild)
	preview.InviteURL = "https://discord.gg/" + strings.Repeat("a", 32)

	encoded, err := json.Marshal(previewEmbed(&worstCase, preview, true))
	if err != nil {
		return err
	}
	if over := len(encoded) - MaxPreviewBytes; over > 0 {
		return fmt.Errorf("the custom text and buttons are too long to fit in a Discord preview, shorten them by about %d characters", over)
	}
	return nil
}

// checkLinks rejects server invites, previews of other servers, and links flagged as scams or unsafe.
// It fails closed, so a link is rejected when the lookups can't be performed.
func checkLinks(text string, guildID int64) error {
	if ContainsOtherServerPreviewLink(text, guildID) {
		return errors.New("links to other servers' previews aren't allowed")
	}

	if !common.LinkRegex.MatchString(text) {
		return nil
	}

	if common.ContainsInvite(text, true, true) != nil {
		return errors.New("invites to servers aren't allowed, the Join button already links to your server")
	}

	scam, err := antiphishing.CheckMessageForPhishingDomains(text)
	if err != nil {
		return errors.New("couldn't check the links right now, try again in a bit")
	}
	if scam != "" {
		return errors.New("the link " + scam + " is flagged as a scam")
	}

	threat, err := safebrowsing.CheckString(text)
	if err != nil {
		return errors.New("couldn't check the links right now, try again in a bit")
	}
	if threat != nil {
		return errors.New("the link " + threat.Pattern + " is flagged as unsafe by Google Safe Browsing")
	}

	return nil
}
