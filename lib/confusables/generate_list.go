//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	ConfusablesURI       = "https://www.unicode.org/Public/security/revision-06/confusables.txt"
	ExtraConfusablesFile = "./extraConfusables.json"

	header = `// This file was generated by go generate; DO NOT EDIT
package %s
`
)

var mishapsReplacer = strings.NewReplacer(
	"vv", "w",
	"rn", "m",
)

var BasicLatin = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x0021, 0x007E, 1},
	},
	R32:         []unicode.Range32{},
	LatinOffset: 5,
}

// Takes a character name as input and then verifies if it's in the list of allowed characters.
func isAllowed(from, to string) bool {
	fromRune := []rune(from)
	toRune := []rune(to)

	if len(fromRune) > 1 {
		return false
	}

	for _, rn := range fromRune {
		if unicode.In(rn, BasicLatin) {
			return false
		}
	}
	for _, rn := range toRune {
		if !unicode.In(rn, BasicLatin) {
			return false
		}
	}

	return true
}

func formatUnicodeIDs(ids string) string {
	var formattedIDs string
	for _, charID := range strings.Split(ids, " ") {
		i, err := strconv.ParseInt(charID, 16, 32)
		if err != nil {
			fmt.Println(err)
			continue
		}

		c := rune(int32(i))

		formattedIDs += string(c)
	}

	return formattedIDs
}

// Fixes a lot of issues with the unicode specification, i.e. m -> rn.
func fixIssuesWithStr(str string) string {
	// Changes characters such as (16) into 16.
	parensRegex := regexp.MustCompile(`\((.+)\)`)
	parensMatches := parensRegex.FindStringSubmatch(str)

	if len(parensMatches) >= 2 {
		str = parensMatches[1]
	}

	// Replaces vv and rn with w and m
	str = mishapsReplacer.Replace(str)

	return str
}

func main() {
	var confusables = make(map[string]string)

	r := regexp.MustCompile(`(?i)([a-zA-Z0-9 ]*) ;	([a-zA-Z0-9 ]*)+ ;	[a-z]{2,}	#\*? \( (.+) →(?: .+ →)* (.+) \) (?:.+)+ → (?:.+)`)

	// Add extra confusables as defined in extraConfusables.json.
	extraConfusables, err := os.OpenFile(ExtraConfusablesFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer extraConfusables.Close()

	decoder := json.NewDecoder(extraConfusables)

	if err := decoder.Decode(&confusables); err != nil {
		fmt.Println(err)
	}

	// Fetch confusables from unicode.org.
	res, err := http.Get(ConfusablesURI)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		txt := scanner.Text()
		matches := r.FindStringSubmatch(txt)

		if len(matches) <= 0 {
			continue
		}

		// Checks if character is latin.
		if allowed := isAllowed(matches[3], matches[4]); !allowed {
			continue
		}

		// Converts unicode IDs into format \U<ID>.
		confusable := formatUnicodeIDs(matches[1])
		targettedCharacter := formatUnicodeIDs(matches[2])
		targettedCharacter = fixIssuesWithStr(targettedCharacter)

		if _, in := confusables[confusable]; in {
			continue
		}

		confusables[confusable] = targettedCharacter
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		return
	}

	fileContent := "var confusables = []string{\n"

	for confusable, confused := range confusables {
		fileContent += fmt.Sprintf("	%s,%s,\n", strconv.Quote(confusable), strconv.Quote(confused))
	}

	fileContent += "}"

	WriteGoFile("confusables_table.go", "confusables", []byte(fileContent))
}

func WriteGoFile(filename, pkg string, b []byte) {
	w, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Could not create file %s: %v", filename, err)
	}
	defer w.Close()

	_, err = fmt.Fprintf(w, header, pkg)
	if err != nil {
		log.Fatalf("Error writing header: %v", err)
	}

	// Strip leading newlines.
	for len(b) > 0 && b[0] == '\n' {
		b = b[1:]
	}
	formatted, err := format.Source(b)

	if err != nil {
		// Print the original buffer even in case of an error so that the
		// returned error can be meaningfully interpreted.
		w.Write(b)
		log.Fatalf("Error formatting file %s: %v", filename, err)
	}

	if _, err := w.Write(formatted); err != nil {
		log.Fatalf("Error writing file %s: %v", filename, err)
	}
}
