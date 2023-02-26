package common

import (
	"unicode"

	"github.com/mtibben/confusables"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r)
}

// stupid name, cant think of a better one atm
func FixText(content string, removeDiacritics, matchConfusables bool) string {
	if !removeDiacritics && !matchConfusables {
		return content
	}

	output := content
	if removeDiacritics {
		t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
		output, _, _ = transform.String(t, output)
	}

	if matchConfusables {
		output = confusables.Skeleton(output)
	}

	return output
}
