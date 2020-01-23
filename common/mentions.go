package common

import (
	"regexp"
	"strconv"
	"strings"
)

const zeroWidthSpace = "​"

var (
	everyoneReplacer    = strings.NewReplacer("@everyone", "@"+zeroWidthSpace+"everyone")
	hereReplacer        = strings.NewReplacer("@here", "@"+zeroWidthSpace+"here")
	patternRoleMentions = regexp.MustCompile("<@&[0-9]*>")

	atReplacer = strings.NewReplacer("@", "@"+zeroWidthSpace)
)

// EscapeSpecialMentions Escapes an everyone mention, adding a zero width space between the '@' and rest
func EscapeSpecialMentions(in string) string {
	return EscapeSpecialMentionsConditional(in, false, false, nil)
}

// EscapeEveryoneHere Escapes an everyone mention, adding a zero width space between the '@' and rest
func EscapeEveryoneHere(s string, escapeEveryone, escapeHere bool) string {
	if escapeEveryone {
		s = everyoneReplacer.Replace(s)
	}

	if escapeHere {
		s = hereReplacer.Replace(s)
	}

	return s
}

// EscapeMentionsFromOutsideSource adds a zws after all @'s
// this is to prevent someone abusing discords filtering of certain unicode characters and creating mentions in various ways
func EscapeMentionsFromOutsideSource(s string) string {
	return atReplacer.Replace(s)
}

// EscapeSpecialMentionsConditional Escapes an everyone mention, adding a zero width space between the '@' and rest
func EscapeSpecialMentionsConditional(s string, allowEveryone, allowHere bool, allowRoles []int64) string {
	if !allowEveryone {
		s = everyoneReplacer.Replace(s)
	}

	if !allowHere {
		s = hereReplacer.Replace(s)
	}

	s = patternRoleMentions.ReplaceAllStringFunc(s, func(x string) string {
		if len(x) < 4 {
			return x
		}

		id := x[3 : len(x)-1]
		parsed, _ := strconv.ParseInt(id, 10, 64)
		if ContainsInt64Slice(allowRoles, parsed) {
			// This role is allowed to be mentioned
			return x
		}

		// Not allowed
		return x[:2] + zeroWidthSpace + x[2:]
	})

	return s
}
