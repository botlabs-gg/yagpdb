//go:generate go run generate_list.go > confusables_table.go

package confusables

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var replacer strings.Replacer

// Create strings replacer accordingly to unicode and "extraConfusables.json".
func Init() {
	fmt.Println("Loading sanitizer...")
	replacer = *strings.NewReplacer(append(confusables, diacritics...)...)
	fmt.Printf("Loaded %d confusables and %d diacritics\n", len(confusables)/2, len(diacritics)/2)
}

// SanitizeText normalizes text and matches confusables.
// i.e. "Ĥéĺĺó" -> "Hello".
func SanitizeText(content string) string {
	content = NormalizeQueryEncodedText(content)
	return replacer.Replace(content)
}

// Normalizes QueryEscaped content in a string.
// Example: "Hello%20World%20%"" will be normalized to "Hello World "
func NormalizeQueryEncodedText(content string) string {
	decoded, err := url.QueryUnescape(content)
	if err != nil {
		decoded = content
	}
	decoded = norm.NFC.String(decoded)
	return decoded
}
