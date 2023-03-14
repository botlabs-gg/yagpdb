package jarowinkler

import (
	"math"
	"strconv"
	"testing"
)

const tolerance = .01

func TestSimilarity(t *testing.T) {
	testCases := []struct {
		a, b string
		want float64
	}{
		{"aa", "a", 0.85},
		{"a", "aa", 0.85},
		{"v", "veryveryverylong", 0.68},
		{"jones", "johnson", 0.83},
		{"fvie", "ten", 0},
		{"henka", "henkan", 0.96},
		{"my string", "my ntrisg", 0.89},
		{"my string", "my tsring", 0.97},
		{"dixon", "dicksonx", 0.81},
		{"dwayne", "duane", 0.84},
		{"martha", "marhta", 0.96},
		{"aaaa", "aaaa", 1},
		{"123", "123", 1},
		{"", "", 1},
		{"", "hi", 0},
		{"abc", "abc", 1},
		{"AA", "a", 0},
	}

	for i, v := range testCases {
		t.Run("Case "+strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := Similarity([]rune(v.a), []rune(v.b))
			if math.Abs(got-v.want) > tolerance {
				t.Errorf("got %.5f, want %.5f", got, v.want)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	testCases := []struct {
		choices       []string
		target        string
		threshold     float64
		caseSensitive bool
		limit         int
		want          []string
	}{
		{[]string{"zac ephron", "kai ephron"}, "zac efron", 0.9, true, 1, []string{"zac ephron"}},
		{[]string{"zac ephron", "kai ephron"}, "zac efron", AdaptiveThreshold, true, 1, []string{"zac ephron"}},
		{[]string{"zac ephron", "kai ephron"}, "zac efron", AdaptiveThreshold, true, -1, []string{"zac ephron", "kai ephron"}},
		{[]string{"britney sphears", "spears britney"}, "britney spears", 0.8, true, 1, []string{"britney sphears"}},
		{[]string{"zac ephron", "zac efron", "britney spheres", "brtney speears"}, "britney spears", 0.8, true, -1, []string{"brtney speears", "britney spheres"}},
		{[]string{"zac ephron", "zac efron", "britney spheres", "brtney speears"}, "britney spears", AdaptiveThreshold, true, -1, []string{"brtney speears", "britney spheres"}},
		{[]string{"reoles", "othermenu", "role", "roles"}, "roels", 0.7, true, 2, []string{"roles", "reoles"}},
		{[]string{"reoles", "othermenu", "role", "roles"}, "roels", 0.7, true, -1, []string{"roles", "reoles", "role"}},
		{[]string{"reoles", "othermenu", "role", "roles"}, "roels", AdaptiveThreshold, true, -1, []string{"roles", "reoles", "role"}},
		{[]string{"zAc ePhrOn", "Kai Ephron"}, "zac EFroN", 0.9, false, 0, nil},
		{[]string{"asd", "bas", "hello", "world"}, "as", AdaptiveThreshold, false, -1, nil},

		{[]string{"zAc ePhrOn", "Kai Ephron"}, "zac EFroN", 0.9, false, 1, []string{"zAc ePhrOn"}},
		{[]string{"zAc ePhrOn", "Kai Ephron"}, "zac efron", AdaptiveThreshold, false, 1, []string{"zAc ePhrOn"}},
	}

	for i, v := range testCases {
		t.Run("Case "+strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := Select(v.choices, v.target, WithThreshold(v.threshold), WithCaseSensitivity(v.caseSensitive), WithLimit(v.limit))
			if len(got) != len(v.want) {
				t.Errorf("got %#v, wanted %#v", got, v.want)
				return
			}
			for y, s := range got {
				if s != v.want[y] {
					t.Errorf("got %#v, wanted %#v", got, v.want)
					return
				}
			}
		})
	}
}
