package messagecreator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

const (
	// Content + Embed
	ModeNormal = "normal"
	// CV2
	ModeComponentsV2 = "componentsv2"

	maxCustomIDLen = 100 // discord limitation
)

const v2Flag = discordgo.MessageFlagsIsComponentsV2

type SendRequest struct {
	ChannelID int64           `json:"channel_id,string"`
	Mode      string          `json:"mode"`
	Payload   json.RawMessage `json:"payload"`
}
type EditRequest struct {
	ChannelID int64           `json:"channel_id,string"`
	MessageID int64           `json:"message_id,string"`
	Mode      string          `json:"mode"`
	Payload   json.RawMessage `json:"payload"`
}
type LoadResponse struct {
	AuthorIsBot bool            `json:"author_is_bot"`
	Mode        string          `json:"mode"`
	Payload     json.RawMessage `json:"payload"`
}

func parsePayload(mode string, raw []byte) (*discordgo.MessageSend, error) {
	msg := &discordgo.MessageSend{}
	if err := json.Unmarshal(raw, msg); err != nil {
		return nil, errors.WithMessage(err, "invalid message JSON")
	}

	msg.Flags = 0
	msg.TTS = false
	msg.Reference = nil
	msg.Files = nil

	switch mode {
	case ModeComponentsV2:
		msg.Embeds = []*discordgo.MessageEmbed{}
		msg.Content = ""
		if len(msg.Components) == 0 {
			return nil, errors.New("a Components V2 message must have at least one component")
		}
		if err := applyTemplatePrefix(msg.Components); err != nil {
			return nil, err
		}
		msg.Flags |= v2Flag
	case ModeNormal:
		// Legacy message: content + embeds + action-row components may all be present together.
		if msg.Content == "" && len(msg.Embeds) == 0 && len(msg.Components) == 0 {
			return nil, errors.New("message must have content, an embed, or components")
		}
		if len(msg.Components) > 0 {
			if err := applyTemplatePrefix(msg.Components); err != nil {
				return nil, err
			}
		}
		// Non-nil slices so an edit explicitly clears removed embeds/components.
		if msg.Embeds == nil {
			msg.Embeds = []*discordgo.MessageEmbed{}
		}
		if msg.Components == nil {
			msg.Components = []discordgo.TopLevelComponent{}
		}
	default:
		return nil, errors.New("unknown message mode")
	}

	if err := validateMessage(mode, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func rcLen(s string) int { return utf8.RuneCountInString(s) }

func validURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "attachment://")
}

func countComponents(comps []discordgo.TopLevelComponent) int {
	n := 0
	for _, c := range comps {
		n++
		switch v := c.(type) {
		case *discordgo.ActionsRow:
			n += len(v.Components)
		case *discordgo.Container:
			n += countComponents(v.Components)
		case *discordgo.Section:
			n += len(v.Components)
			if v.Accessory != nil {
				n++
			}
		}
	}
	return n
}

func validateMessage(mode string, msg *discordgo.MessageSend) error {
	if rcLen(msg.Content) > 2000 {
		return errors.New("message content exceeds 2000 characters")
	}
	if len(msg.Embeds) > 10 {
		return errors.New("a message can have at most 10 embeds")
	}
	for i, e := range msg.Embeds {
		if err := validateEmbed(i+1, e); err != nil {
			return err
		}
	}
	if mode == ModeComponentsV2 && countComponents(msg.Components) > 40 {
		return errors.New("a Components V2 message can have at most 40 components in total")
	}
	if mode == ModeNormal {
		rows := 0
		for _, c := range msg.Components {
			if _, ok := c.(*discordgo.ActionsRow); ok {
				rows++
			}
		}
		if rows > 5 {
			return errors.New("a message can have at most 5 action rows")
		}
	}
	return validateComponents(msg.Components)
}

func validateEmbed(n int, e *discordgo.MessageEmbed) error {
	if e == nil {
		return nil
	}
	total := rcLen(e.Title) + rcLen(e.Description)
	if rcLen(e.Title) > 256 {
		return fmt.Errorf("embed %d: title exceeds 256 characters", n)
	}
	if rcLen(e.Description) > 4096 {
		return fmt.Errorf("embed %d: description exceeds 4096 characters", n)
	}
	if e.Author != nil {
		if rcLen(e.Author.Name) > 256 {
			return fmt.Errorf("embed %d: author name exceeds 256 characters", n)
		}
		total += rcLen(e.Author.Name)
		if e.Author.IconURL != "" && !validURL(e.Author.IconURL) {
			return fmt.Errorf("embed %d: author icon is not a valid URL", n)
		}
		if e.Author.URL != "" && !validURL(e.Author.URL) {
			return fmt.Errorf("embed %d: author URL is not valid", n)
		}
	}
	if e.Footer != nil {
		if rcLen(e.Footer.Text) > 2048 {
			return fmt.Errorf("embed %d: footer text exceeds 2048 characters", n)
		}
		total += rcLen(e.Footer.Text)
		if e.Footer.IconURL != "" && !validURL(e.Footer.IconURL) {
			return fmt.Errorf("embed %d: footer icon is not a valid URL", n)
		}
	}
	if len(e.Fields) > 25 {
		return fmt.Errorf("embed %d: at most 25 fields", n)
	}
	for fi, f := range e.Fields {
		if f.Name == "" || f.Value == "" {
			return fmt.Errorf("embed %d field %d: both name and value are required", n, fi+1)
		}
		if rcLen(f.Name) > 256 {
			return fmt.Errorf("embed %d field %d: name exceeds 256 characters", n, fi+1)
		}
		if rcLen(f.Value) > 1024 {
			return fmt.Errorf("embed %d field %d: value exceeds 1024 characters", n, fi+1)
		}
		total += rcLen(f.Name) + rcLen(f.Value)
	}
	if total > 6000 {
		return fmt.Errorf("embed %d: total text exceeds 6000 characters", n)
	}
	if e.URL != "" && !validURL(e.URL) {
		return fmt.Errorf("embed %d: URL is not valid", n)
	}
	if e.Image != nil && e.Image.URL != "" && !validURL(e.Image.URL) {
		return fmt.Errorf("embed %d: image URL is not valid", n)
	}
	if e.Thumbnail != nil && e.Thumbnail.URL != "" && !validURL(e.Thumbnail.URL) {
		return fmt.Errorf("embed %d: thumbnail URL is not valid", n)
	}
	return nil
}

func validateComponents(comps []discordgo.TopLevelComponent) error {
	for _, c := range comps {
		switch v := c.(type) {
		case *discordgo.ActionsRow:
			if len(v.Components) == 0 {
				return errors.New("an action row must contain at least one component")
			}
			if len(v.Components) > 5 {
				return errors.New("an action row can have at most 5 components")
			}
			hasSelect := false
			for _, ic := range v.Components {
				if ic.Type() != discordgo.ButtonComponent {
					hasSelect = true
				}
			}
			if hasSelect && len(v.Components) > 1 {
				return errors.New("a select menu cannot share an action row with other components")
			}
			for _, ic := range v.Components {
				if err := validateInteractive(ic); err != nil {
					return err
				}
			}
		case *discordgo.Container:
			if len(v.Components) == 0 {
				return errors.New("a container must contain at least one component")
			}
			if err := validateComponents(v.Components); err != nil {
				return err
			}
		case *discordgo.Section:
			hasText := false
			for _, sc := range v.Components {
				if td, ok := sc.(*discordgo.TextDisplay); ok && td.Content != "" {
					hasText = true
				}
			}
			if !hasText {
				return errors.New("a section needs at least one non-empty text component")
			}
			if v.Accessory == nil {
				return errors.New("a section needs an accessory")
			}
			if b, ok := v.Accessory.(*discordgo.Button); ok {
				if err := validateButton(b); err != nil {
					return err
				}
			}
			if th, ok := v.Accessory.(*discordgo.Thumbnail); ok {
				if th.Media.URL == "" {
					return errors.New("a section thumbnail needs an image URL")
				}
				if !validURL(th.Media.URL) {
					return errors.New("section thumbnail URL is not valid")
				}
			}
		case *discordgo.TextDisplay:
			if v.Content == "" {
				return errors.New("a text component cannot be empty")
			}
			if rcLen(v.Content) > 4000 {
				return errors.New("a text component exceeds 4000 characters")
			}
		case *discordgo.MediaGallery:
			if len(v.Items) == 0 {
				return errors.New("a media gallery needs at least one item")
			}
			for _, it := range v.Items {
				if it.Media.URL == "" {
					return errors.New("a media gallery item needs an image URL")
				}
				if !validURL(it.Media.URL) {
					return errors.New("a media gallery URL must start with http:// or https://")
				}
			}
		}
	}
	return nil
}

func validateInteractive(ic discordgo.InteractiveComponent) error {
	switch v := ic.(type) {
	case *discordgo.Button:
		return validateButton(v)
	case *discordgo.SelectMenu:
		return validateSelect(v)
	}
	return nil
}

func validateButton(b *discordgo.Button) error {
	if b.Style == discordgo.LinkButton {
		if b.URL == "" {
			return errors.New("a link button needs a URL")
		}
		if !validURL(b.URL) {
			return errors.New("a link button URL must start with http:// or https://")
		}
	} else if b.Label == "" && b.Emoji == nil {
		return errors.New("a button needs a label or emoji")
	}
	if rcLen(b.Label) > 80 {
		return errors.New("a button label exceeds 80 characters")
	}
	return nil
}

func validateSelect(m *discordgo.SelectMenu) error {
	if rcLen(m.Placeholder) > 150 {
		return errors.New("a select placeholder exceeds 150 characters")
	}
	if m.MenuType == discordgo.StringSelectMenu || m.MenuType == 0 {
		if len(m.Options) < 1 || len(m.Options) > 25 {
			return errors.New("a text-options select menu needs between 1 and 25 options")
		}
		seen := make(map[string]bool, len(m.Options))
		for i, o := range m.Options {
			if o.Label == "" {
				return fmt.Errorf("select option %d needs a label", i+1)
			}
			if o.Value == "" {
				return fmt.Errorf("select option %d needs a value", i+1)
			}
			if seen[o.Value] {
				return errors.New("select option values must be unique")
			}
			seen[o.Value] = true
		}
	}
	if m.MinValues != nil && (*m.MinValues < 0 || *m.MinValues > 25) {
		return errors.New("select min values must be between 0 and 25")
	}
	if m.MaxValues < 0 || m.MaxValues > 25 {
		return errors.New("select max values must be between 0 and 25")
	}
	if m.MinValues != nil && m.MaxValues > 0 && *m.MinValues > m.MaxValues {
		return errors.New("select min values cannot exceed max values")
	}
	return nil
}

func applyTemplatePrefix(comps []discordgo.TopLevelComponent) error {
	counter := 0
	return prefixComponents(comps, &counter)
}

func prefixComponents(comps []discordgo.TopLevelComponent, counter *int) error {
	for _, c := range comps {
		switch v := c.(type) {
		case *discordgo.ActionsRow:
			for _, inner := range v.Components {
				if err := prefixInteractive(inner, counter); err != nil {
					return err
				}
			}
		case *discordgo.Container:
			if err := prefixComponents(v.Components, counter); err != nil {
				return err
			}
		case *discordgo.Section:
			if acc, ok := v.Accessory.(discordgo.InteractiveComponent); ok {
				if err := prefixInteractive(acc, counter); err != nil {
					return err
				}
			}
		case *discordgo.Label:
			if err := prefixInteractive(v.Component, counter); err != nil {
				return err
			}
		}
	}
	return nil
}

func prefixInteractive(c discordgo.InteractiveComponent, counter *int) error {
	switch v := c.(type) {
	case *discordgo.Button:
		if v.Style == discordgo.LinkButton {
			v.CustomID = "" // link buttons must not have a custom id
			return nil
		}
		return setPrefixedID(&v.CustomID, counter)
	case *discordgo.SelectMenu:
		return setPrefixedID(&v.CustomID, counter)
	case *discordgo.TextInput:
		return setPrefixedID(&v.CustomID, counter)
	case *discordgo.RadioGroup:
		return setPrefixedID(&v.CustomID, counter)
	case *discordgo.CheckboxGroup:
		return setPrefixedID(&v.CustomID, counter)
	case *discordgo.Checkbox:
		return setPrefixedID(&v.CustomID, counter)
	}
	return nil
}

func setPrefixedID(id *string, counter *int) error {
	if *id == "" {
		*id = templates.TemplateCustomIDPrefix + strconv.Itoa(*counter)
	}
	*counter++
	if rcLen(*id) > maxCustomIDLen {
		return errors.Errorf("custom id %q is too long (max %d characters including the %q prefix)", *id, maxCustomIDLen, templates.TemplateCustomIDPrefix)
	}
	return nil
}

var messageLinkRegex = regexp.MustCompile(`(?:https?://)?(?:\w+\.)?discord(?:app)?\.com/channels/(\d+)/(\d+)/(\d+)`)

// parseMessageLink extracts the guild, channel and message IDs from a Discord message link.
func parseMessageLink(link string) (guildID, channelID, messageID int64, err error) {
	m := messageLinkRegex.FindStringSubmatch(strings.TrimSpace(link))
	if m == nil {
		return 0, 0, 0, errors.New("not a valid Discord message link")
	}
	guildID, _ = strconv.ParseInt(m[1], 10, 64)
	channelID, _ = strconv.ParseInt(m[2], 10, 64)
	messageID, _ = strconv.ParseInt(m[3], 10, 64)
	return guildID, channelID, messageID, nil
}
