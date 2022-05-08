// Package jarowinkler implements fuzzy matching based on the Jaro-Winkler metric.
package jarowinkler

import (
	"sort"
	"strings"
)

const (
	winklerThreshold  = 0.7
	winklerPrefixSize = 4
)

// Similarity computes the Jaro-Winkler similarity between a and b. The result
// falls in the range 0, indicating no match, to 1, indicating a perfect match.
func Similarity(a, b []rune) float64 {
	if len(a) == 0 {
		if len(b) == 0 {
			return 1
		}
		return 0
	}

	window := max(max(len(a), len(b))/2-1, 0)
	matchedA, matchedB := make([]bool, len(a)), make([]bool, len(b))
	common := 0
	for i := 0; i < len(a); i++ {
		for j := max(i-window, 0); j < min(i+window+1, len(b)); j++ {
			if matchedB[j] {
				continue
			}
			if a[i] != b[j] {
				continue
			}
			matchedA[i] = true
			matchedB[j] = true
			common++
			break
		}
	}

	if common == 0 {
		return 0
	}

	halfTransposed := 0
	k := 0
	for i := 0; i < len(a); i++ {
		if !matchedA[i] {
			continue
		}
		for !matchedB[k] {
			k++
		}
		if a[i] != b[k] {
			halfTransposed++
		}
		k++
	}

	transposed := halfTransposed / 2
	weight := (float64(common)/float64(len(a)) + float64(common)/float64(len(b)) + float64(common-transposed)/float64(common)) / 3
	if weight <= winklerThreshold {
		return weight
	}

	lcpMax := min(min(winklerPrefixSize, len(a)), len(b))
	lcp := 0
	for lcp < lcpMax && a[lcp] == b[lcp] {
		lcp++
	}
	return weight + 0.1*float64(lcp)*(1-weight)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Select selects items from the set of available choices based on similarity to
// the target according to the Jaro-Winkler metric. The selections are sorted in
// descending order with respect to their similarity to the target string.
//
// By default,
//  - Choices that are not sufficiently similar to the string are discarded based
//    on an adaptive threshold. If a custom threshold is desired, use the WithThreshold
//    option.
//  - The number of choices that are returned is unlimited; to limit the
//    number of returned results, use the WithLimit option.
//  - The selection process is case-insensitive; to change this, use the WithCaseInsensitivity
//    option.
func Select(choices []string, target string, setters ...SelectOpt) []string {
	opts := selectOpts{threshold: AdaptiveThreshold, limit: -1, caseSensitive: false}
	for _, setter := range setters {
		setter(&opts)
	}

	targetRunes := getRunes(target, opts.caseSensitive)
	threshold := opts.threshold
	if threshold == AdaptiveThreshold {
		threshold = computeAdaptiveThreshold(len(targetRunes))
	}

	type selection struct {
		val        string
		similarity float64
	}
	selections := make([]selection, 0, len(choices))
	for _, choice := range choices {
		if s := Similarity(targetRunes, getRunes(choice, opts.caseSensitive)); s >= threshold {
			selections = append(selections, selection{choice, s})
		}
	}

	sort.Slice(selections, func(i, j int) bool {
		return selections[i].similarity > selections[j].similarity
	})

	keep := opts.limit
	if opts.limit < 0 || opts.limit >= len(selections) {
		keep = len(selections)
	}
	res := make([]string, keep)
	for i := 0; i < keep; i++ {
		res[i] = selections[i].val
	}
	return res
}

func getRunes(s string, caseSensitive bool) []rune {
	if caseSensitive {
		return []rune(s)
	}
	return []rune(strings.ToLower(s))
}

// AdaptiveThreshold indicates that the threshold should be computed dynamically
// based on the input length.
const AdaptiveThreshold = -1

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

// A SelectOpt enables fine-tuning of the selection process.
type SelectOpt func(*selectOpts)

type selectOpts struct {
	threshold     float64
	limit         int
	caseSensitive bool
}

// WithThreshold returns an option that discards choices such that the
// Jaro-Winkler similarity to the target string is less than threshold. If
// threshold == AdaptiveThreshold, an adaptive threshold based on the length of
// the target string will be used.
func WithThreshold(threshold float64) SelectOpt {
	return func(opts *selectOpts) {
		opts.threshold = threshold
	}
}

// WithLimit returns an option that limits the number of returned results to at
// most limit. If limit < 0, the number of results is unlimited.
func WithLimit(limit int) SelectOpt {
	return func(opts *selectOpts) {
		opts.limit = limit
	}
}

// WithCaseSensitivity returns an option that enables/disables case-sensitive
// matching based on the enabled parameter.
func WithCaseSensitivity(enabled bool) SelectOpt {
	return func(opts *selectOpts) {
		opts.caseSensitive = enabled
	}
}
