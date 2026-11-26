package automod

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// foldKey maps every rune to the smallest rune of its simple case folding orbit,
// so foldKey(a) == foldKey(b) exactly when strings.EqualFold(a, b).
func foldKey(s string) string {
	needsWork := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf || ('a' <= c && c <= 'z') {
			needsWork = true
			break
		}
	}
	if !needsWork {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < utf8.RuneSelf {
			if 'a' <= r && r <= 'z' {
				r -= 'a' - 'A'
			}
			b.WriteRune(r)
			continue
		}

		lowest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < lowest {
				lowest = f
			}
		}
		b.WriteRune(lowest)
	}
	return b.String()
}

type wordSet map[string]struct{}

func newWordSet(words []string) wordSet {
	set := make(wordSet, len(words))
	for _, w := range words {
		set[foldKey(w)] = struct{}{}
	}
	return set
}

func (s wordSet) contains(word string) bool {
	_, ok := s[foldKey(word)]
	return ok
}

// wordListTriggered returns the first field that triggers a denylist (a field in the set)
// or an allowlist (a field not in the set).
func wordListTriggered(set wordSet, fields []string, blacklist bool) (word string, triggered bool) {
	for _, f := range fields {
		if set.contains(f) == blacklist {
			return f, true
		}
	}
	return "", false
}

const maxReasonWordLength = 50

// wordListMatchDetail describes a word list match for the punishment reason.
func wordListMatchDetail(guildID, listID int64, word string, blacklist bool) string {
	listName := "unknown list"
	if list, err := FindFetchGuildList(guildID, listID); err == nil {
		listName = list.Name
	}

	relation := "in"
	if !blacklist {
		relation = "not in"
	}

	return fmt.Sprintf("`%s` %s list `%s`", reasonCodeText(word), relation, reasonCodeText(listName))
}

func reasonCodeText(s string) string {
	s = strings.ReplaceAll(s, "`", "'")
	if utf8.RuneCountInString(s) > maxReasonWordLength {
		s = string([]rune(s)[:maxReasonWordLength]) + "..."
	}
	return s
}
