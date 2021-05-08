package fuzzy

import (
	"math"
	"sort"
	"unicode"
)

const (
	WinklerThreshold  = 0.7
	WinklerPrefixSize = 4
)

// Similarity computes the Jaro-Winkler similarity between two strings with
// configurable case sensitivity. The result falls in the range 0 (no match) to
// 1 (perfect match).
//
// It is a translation of the code provided here:
// https://stackoverflow.com/a/19165108
func Similarity(s1, s2 []rune, caseSensitive bool) float64 {
	s1Len := len(s1)
	s2Len := len(s2)
	if s1Len == 0 {
		if s2Len == 0 {
			return 1
		}
		return 0
	}

	searchRange := math.Max(0, math.Max(float64(s1Len), float64(s2Len))/2-1)

	matched1 := make([]bool, s1Len)
	matched2 := make([]bool, s2Len)

	common := 0
	for i := 0; i < s1Len; i++ {
		start := int(math.Max(0, float64(i)-searchRange))
		end := int(math.Min(float64(i)+searchRange+float64(1), float64(s2Len)))
		for j := start; j < end; j++ {
			if matched2[j] {
				continue
			}
			if !runesEq(s1[i], s2[j], caseSensitive) {
				continue
			}

			matched1[i] = true
			matched2[j] = true
			common++
			break
		}
	}

	if common == 0 {
		return 0
	}

	numHalfTransposed := 0
	k := 0
	for i := 0; i < s1Len; i++ {
		if !matched1[i] {
			continue
		}
		for !matched2[k] {
			k++
		}
		if !runesEq(s1[i], s2[k], caseSensitive) {
			numHalfTransposed++
		}
		k++
	}

	numTransposed := numHalfTransposed / 2
	weight := (float64(common)/float64(s1Len) + float64(common)/float64(s2Len) + float64(common-numTransposed)/float64(common)) / 3
	if weight <= WinklerThreshold {
		return weight // don't apply Winkler modification unless the strings are reasonably similar
	}

	max := int(math.Min(WinklerPrefixSize, math.Min(float64(s1Len), float64(s2Len))))
	pos := 0
	for pos < max && runesEq(s1[pos], s2[pos], caseSensitive) {
		pos++
	}

	if pos == 0 {
		return weight
	}

	return weight + 0.1*float64(pos)*(1-weight)
}

func runesEq(r1, r2 rune, caseSensitive bool) bool {
	if caseSensitive {
		return r1 == r2
	}
	// unfortunate, but no real better way in this context
	return unicode.ToLower(r1) == unicode.ToLower(r2)
}

// AdaptiveThreshold is a special value indicating that Select should compute the
// optimal threshold automatically.
const AdaptiveThreshold = -1

// Selection is a single selection returned from the Select* methods.
type Selection struct {
	Value    string  // The string selected.
	Distance float64 // The Jaro-Winkler distance between the query and string.
}

// SelectN returns at most N strings from options for which the Jaro-Winkler
// distance between the query and the option is greater than or equal to
// threshold. The selections are sorted in descending order by their
// Jaro-Winkler distance. If n < 0, there is no limit on the number of returned
// selections. If threshold == AdaptiveThreshold, an optimal threshold will be
// computed automatically.
func SelectN(query string, options []string, threshold float64, caseSensitive bool, n int) []*Selection {
	if n == 0 {
		return nil
	}

	queryRunes := []rune(query)
	if threshold == AdaptiveThreshold {
		threshold = computeAdaptiveThreshold(len(queryRunes))
	}

	var selections []*Selection
	for _, option := range options {
		if d := Similarity(queryRunes, []rune(option), caseSensitive); d >= threshold {
			selections = append(selections, &Selection{Value: option, Distance: d})
		}
	}

	sort.Slice(selections, func(i, j int) bool {
		return selections[i].Distance > selections[j].Distance
	})
	if n < 0 || len(selections) <= n {
		return selections
	}
	return selections[:n]
}

// SelectAll returns all the options for which the Jaro-Winkler distance between
// the query and the option is greater than or equal to threshold.
// If threshold == AdaptiveThreshold, an optimal threshold will be computed
// automatically.
//
// It is equivalent to SelectN with a limit of -1.
func SelectAll(query string, options []string, threshold float64, caseSensitive bool) []*Selection {
	return SelectN(query, options, threshold, caseSensitive, -1)
}

func computeAdaptiveThreshold(len int) float64 {
	switch {
	case len <= 3:
		return 1
	case len <= 6:
		return 0.8
	case len <= 12:
		return 0.7
	default:
		return 0.6
	}
}
