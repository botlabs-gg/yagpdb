package automod

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestFoldKeyMatchesEqualFoldPerRune(t *testing.T) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		s := string(r)
		key := foldKey(s)
		if !strings.EqualFold(s, key) {
			t.Fatalf("foldKey(%q) = %q, not EqualFold to input", s, key)
		}
		folded := string(unicode.SimpleFold(r))
		if foldKey(folded) != key {
			t.Fatalf("foldKey(%q) = %q but foldKey(%q) = %q", s, key, folded, foldKey(folded))
		}
	}
}

func TestFoldKeyMatchesEqualFoldStrings(t *testing.T) {
	words := []string{
		"", "hello", "HELLO", "HeLLo", "hell", "hellö", "HELLÖ", "straße", "STRASSE", "ſtop", "STOP",
		"Kelvin", "kelvin", "ǅ", "ǆ", "Ǆ", "Σίσυφος", "ΣΊΣΥΦΟΣ", "σίσυφοσ", "ς", "\xff", "�",
		"a\xffb", "A�B", "İstanbul", "istanbul", "ıstanbul", "word1", "WORD1", "🙂", "🙂x", "🙂X",
	}
	for _, a := range words {
		for _, b := range words {
			want := strings.EqualFold(a, b)
			got := foldKey(a) == foldKey(b)
			if want != got {
				t.Errorf("%q vs %q: EqualFold=%v foldKey equal=%v", a, b, want, got)
			}
		}
	}
}

func naiveWordListTriggered(list, fields []string, blacklist bool) bool {
	for _, mf := range fields {
		contained := false
		for _, w := range list {
			if strings.EqualFold(mf, w) {
				if blacklist {
					return true
				}
				contained = true
				break
			}
		}
		if !blacklist && !contained {
			return true
		}
	}
	return false
}

func TestWordListTriggeredMatchesNaive(t *testing.T) {
	lists := [][]string{
		nil,
		{"bad", "Wörd", "ſecret", "Kill"},
		{"hello", "world"},
	}
	messages := []string{
		"", "hello world", "HELLO there", "this is BAD", "WÖRD", "secret", "SECRET", "kill", "KILL",
		"b*a*d w-o-r-d", "nothing here", "hello\xffworld",
	}
	for _, list := range lists {
		set := newWordSet(list)
		for _, msg := range messages {
			prepared := PrepareMessageForWordCheck(msg)
			for _, fields := range [][]string{strings.Fields(prepared), strings.Fields(msg)} {
				for _, blacklist := range []bool{true, false} {
					want := naiveWordListTriggered(list, fields, blacklist)
					_, got := wordListTriggered(set, fields, blacklist)
					if want != got {
						t.Errorf("list=%q fields=%q blacklist=%v: want %v got %v", list, fields, blacklist, want, got)
					}
				}
			}
		}
	}
}
