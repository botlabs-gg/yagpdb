//go:generate go run generate_list.go > confusables_table.go

package confusables

import (
	"fmt"
	"strings"
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
	// Normalize string.
	return replacer.Replace(content)
}
