package dcmd

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type Response interface {
	// Channel, session, command etc can all be found in this context
	Send(data *Data) ([]*discordgo.Message, error)
}

func SendResponseInterface(data *Data, reply interface{}, escapeEveryoneMention bool) ([]*discordgo.Message, error) {

	allowedMentions := discordgo.AllowedMentions{}
	if !escapeEveryoneMention {
		// Legacy behaviour
		allowedMentions.Parse = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeUsers}
	}

	return data.SendFollowupMessage(reply, allowedMentions)
}

// Temporary response deletes the inner response after Duration
type TemporaryResponse struct {
	Response       interface{}
	Duration       time.Duration
	EscapeEveryone bool
}

func NewTemporaryResponse(d time.Duration, inner interface{}, escapeEveryoneMention bool) *TemporaryResponse {
	return &TemporaryResponse{
		Duration: d, Response: inner,

		EscapeEveryone: escapeEveryoneMention,
	}
}

func (t *TemporaryResponse) Send(data *Data) ([]*discordgo.Message, error) {

	msgs, err := SendResponseInterface(data, t.Response, t.EscapeEveryone)
	if err != nil {
		return nil, err
	}

	time.AfterFunc(t.Duration, func() {
		// do a bulk if 2 or more
		if len(msgs) > 1 {
			ids := make([]int64, len(msgs))
			for i, m := range msgs {
				ids[i] = m.ID
			}
			data.Session.ChannelMessagesBulkDelete(data.ChannelID, ids)
		} else {
			data.Session.ChannelMessageDelete(data.ChannelID, msgs[0].ID)
		}
	})
	return msgs, nil
}

// SplitSendMessage uses SplitString to make sure each message is within 2k characters and splits at last newline before that (if possible)
func SplitSendMessage(data *Data, contents string, allowedMentions discordgo.AllowedMentions) ([]*discordgo.Message, error) {
	result := make([]*discordgo.Message, 0, 1)

	split := SplitString(contents, 2000)
	for _, v := range split {
		var err error
		var m *discordgo.Message
		switch data.TriggerType {
		case TriggerTypeSlashCommands:
			m, err = data.Session.CreateFollowupMessage(data.SlashCommandTriggerData.Interaction.ApplicationID, data.SlashCommandTriggerData.Interaction.Token, &discordgo.WebhookParams{
				Content:         v,
				AllowedMentions: &allowedMentions,
			})
		default:
			m, err = data.Session.ChannelMessageSendComplex(data.ChannelID, &discordgo.MessageSend{
				Content:         v,
				AllowedMentions: allowedMentions,
			})
		}

		if err != nil {
			return result, err
		}

		result = append(result, m)
	}

	return result, nil
}

// SplitString uses StrSplitNext to split a string at the last newline before maxLen, throwing away leading and ending whitespaces in the process
func SplitString(s string, maxLen int) []string {
	result := make([]string, 0, 1)

	rest := s
	for {
		if strings.TrimSpace(rest) == "" {
			break
		}

		var split string
		split, rest = StrSplitNext(rest, maxLen)

		split = strings.TrimSpace(split)
		if split == "" {
			continue
		}

		result = append(result, split)
	}

	return result
}

// StrSplitNext Will split "s" before runecount at last possible newline, whitespace or just at "runecount" if there is no whitespace
// If the runecount in "s" is less than "runeCount" then "last" will be zero
func StrSplitNext(s string, runeCount int) (split, rest string) {
	if utf8.RuneCountInString(s) <= runeCount {
		return s, ""
	}

	_, beforeIndex := RuneByIndex(s, runeCount)
	firstPart := s[:beforeIndex]

	// Split at newline if possible
	foundWhiteSpace := false
	lastIndex := strings.LastIndex(firstPart, "\n")
	if lastIndex == -1 {
		// No newline, check for any possible whitespace then
		lastIndex = strings.LastIndexFunc(firstPart, func(r rune) bool {
			return unicode.In(r, unicode.White_Space)
		})
		if lastIndex == -1 {
			lastIndex = beforeIndex
		} else {
			foundWhiteSpace = true
		}
	} else {
		foundWhiteSpace = true
	}

	// Remove the whitespace we split at if any
	if foundWhiteSpace {
		_, rLen := utf8.DecodeRuneInString(s[lastIndex:])
		rest = s[lastIndex+rLen:]
	} else {
		rest = s[lastIndex:]
	}

	split = s[:lastIndex]

	return
}

// RuneByIndex Returns the string index from the rune position
// Panics if utf8.RuneCountInString(s) <= runeIndex or runePos < 0
func RuneByIndex(s string, runePos int) (rune, int) {
	sLen := utf8.RuneCountInString(s)
	if sLen <= runePos || runePos < 0 {
		panic("runePos is out of bounds")
	}

	i := 0
	last := rune(0)
	for k, r := range s {
		if i == runePos {
			return r, k
		}
		i++
		last = r
	}
	return last, i
}
