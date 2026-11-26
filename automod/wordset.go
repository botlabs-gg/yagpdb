package automod

import (
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

// wordListTriggered reports whether fields trigger a denylist (any field in the set)
// or an allowlist (any field not in the set).
func wordListTriggered(set wordSet, fields []string, blacklist bool) bool {
	for _, f := range fields {
		if set.contains(f) == blacklist {
			return true
		}
	}
	return false
}
