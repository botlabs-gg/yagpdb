package common

import (
	"unicode"

	"github.com/mtibben/confusables"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var transformer = transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)

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
		output, _, _ = transform.String(transformer, output)
	}

	if matchConfusables {
		output = confusables.Skeleton(output)
	}

	return output
}
