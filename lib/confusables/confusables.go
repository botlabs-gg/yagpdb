//go:generate go run generate_list.go > confusables_table.go

package confusables

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Replaces characters in the Unicode Table "Marks Nonspaced" and removes / replaces them.
var transformer = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
var replacer strings.Replacer

// Create strings replacer accordingly to unicode and "extraConfusables.json".
func Init() {
	fmt.Printf("Loading confusables..")
	replacer = *strings.NewReplacer(confusables...)
	fmt.Printf("Loaded %d confusables\n", len(confusables)/2)
}

// SanitizeText normalizes text and matches confusables.
// i.e. "Ĥéĺĺó" -> "Hello".
func SanitizeText(content string) string {
	// Normalize string
	output, _, _ := transform.String(transformer, content)

	// Match confusables
	output = replacer.Replace(output)

	return output
}
