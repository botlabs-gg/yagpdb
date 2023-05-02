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
var confusablesReplacer strings.Replacer
var diacriticReplacer strings.Replacer

// Create strings replacer accordingly to unicode and "extraConfusables.json".
func Init() {
	fmt.Println("Loading confusables..")
	confusablesReplacer = *strings.NewReplacer(confusables...)
	fmt.Printf("Loaded %d confusables\n", len(confusables)/2)

	fmt.Println("Loading diacritics..")
	diacriticReplacer = *strings.NewReplacer(diacritics...)
	fmt.Printf("Loaded %d confusables\n", len(diacritics)/2)
}

// SanitizeText normalizes text and matches confusables.
// i.e. "Ĥéĺĺó" -> "Hello".
func SanitizeText(content string) string {
	// Normalize string.
	output := diacriticReplacer.Replace(content)

	// Match confusables.
	output = confusablesReplacer.Replace(output)

	return output
}
