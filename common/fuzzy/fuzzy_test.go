package fuzzy

import (
	"math"
	"strconv"
	"testing"
)

const tolerance = .01

func TestSimilarity(t *testing.T) {
	testCases := []struct {
		s1, s2        string
		caseSensitive bool
		want          float64
	}{
		{"aa", "a", true, 0.85},
		{"a", "aa", true, 0.85},
		{"v", "veryveryverylong", true, 0.68},
		{"jones", "johnson", true, 0.83},
		{"fvie", "ten", true, 0},
		{"henka", "henkan", true, 0.96},
		{"my string", "my ntrisg", true, 0.89},
		{"my string", "my tsring", true, 0.97},
		{"dixon", "dicksonx", true, 0.81},
		{"dwayne", "duane", true, 0.84},
		{"martha", "marhta", true, 0.96},
		{"aaaa", "aaaa", true, 1},
		{"123", "123", true, 1},
		{"", "", true, 1},
		{"", "hi", true, 0},
		{"abc", "abc", true, 1},
		{"AA", "a", true, 0},

		{"AA", "a", false, 0.85},
		{"mY string", "my ntrisg", false, 0.89},
		{"jOnEs", "jOhNsON", false, 0.83},
	}

	for i, v := range testCases {
		t.Run("Case "+strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := Similarity([]rune(v.s1), []rune(v.s2), v.caseSensitive)
			if math.Abs(got-v.want) > tolerance {
				t.Errorf("got %.5f, want %.5f", got, v.want)
			}
		})
	}
}

func selectionsToStrings(selections []*Selection) []string {
	res := make([]string, len(selections))
	for i, selection := range selections {
		res[i] = selection.Value
	}
	return res
}

func TestSelectN(t *testing.T) {
	testCases := []struct {
		query         string
		options       []string
		threshold     float64
		caseSensitive bool
		n             int
		want          []string
	}{
		{"zac efron", []string{"zac ephron", "kai ephron"}, 0.9, true, 1, []string{"zac ephron"}},
		{"zac efron", []string{"zac ephron", "kai ephron"}, AdaptiveThreshold, true, 1, []string{"zac ephron"}},
		{"zac efron", []string{"zac ephron", "kai ephron"}, AdaptiveThreshold, true, -1, []string{"zac ephron", "kai ephron"}},
		{"britney spears", []string{"britney sphears", "spears britney"}, 0.8, true, 1, []string{"britney sphears"}},
		{"britney spears", []string{"zac ephron", "zac efron", "britney spheres", "brtney speears"}, 0.8, true, -1, []string{"brtney speears", "britney spheres"}},
		{"britney spears", []string{"zac ephron", "zac efron", "britney spheres", "brtney speears"}, AdaptiveThreshold, true, -1, []string{"brtney speears", "britney spheres"}},
		{"roels", []string{"reoles", "othermenu", "role", "roles"}, 0.7, true, 2, []string{"roles", "reoles"}},
		{"roels", []string{"reoles", "othermenu", "role", "roles"}, 0.7, true, -1, []string{"roles", "reoles", "role"}},
		{"roels", []string{"reoles", "othermenu", "role", "roles"}, AdaptiveThreshold, true, -1, []string{"roles", "reoles", "role"}},
		{"zac EFroN", []string{"zAc ePhrOn", "Kai Ephron"}, 0.9, false, 0, nil},
		{"as", []string{"asd", "bas", "hello", "world"}, AdaptiveThreshold, false, -1, nil},

		{"zac EFroN", []string{"zAc ePhrOn", "Kai Ephron"}, 0.9, false, 1, []string{"zAc ePhrOn"}},
		{"zac efron", []string{"zAc ePhrOn", "Kai Ephron"}, AdaptiveThreshold, false, 1, []string{"zAc ePhrOn"}},
	}

	for i, v := range testCases {
		t.Run("Case "+strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := SelectN(v.query, v.options, v.threshold, v.caseSensitive, v.n)
			if len(got) != len(v.want) {
				t.Errorf("got %#v, wanted %#v", selectionsToStrings(got), v.want)
				return
			}
			for y, s := range got {
				if s.Value != v.want[y] {
					t.Errorf("got %#v, wanted %#v", selectionsToStrings(got), v.want)
					return
				}
			}
		})
	}
}

func TestSelectAll(t *testing.T) {
	testCases := []struct {
		query         string
		options       []string
		threshold     float64
		caseSensitive bool
		want          []string
	}{
		{"zac efron", []string{"zac ephron", "kai ephron"}, AdaptiveThreshold, true, []string{"zac ephron", "kai ephron"}},
		{"britney spears", []string{"zac ephron", "zac efron", "britney spheres", "brtney speears"}, 0.8, true, []string{"brtney speears", "britney spheres"}},
		{"britney spears", []string{"zac ephron", "zac efron", "britney spheres", "brtney speears"}, AdaptiveThreshold, true, []string{"brtney speears", "britney spheres"}},
		{"roels", []string{"reoles", "othermenu", "role", "roles"}, 0.7, true, []string{"roles", "reoles", "role"}},
		{"roels", []string{"reoles", "othermenu", "role", "roles"}, AdaptiveThreshold, true, []string{"roles", "reoles", "role"}},
		{"roels", []string{"hello", "world", "asdf", "anotherrolemenu"}, AdaptiveThreshold, true, nil},

		{"zac EFroN", []string{"zAc ePhrOn", "Kai Ephron"}, 0.9, false, []string{"zAc ePhrOn"}},
		{"zac efron", []string{"zAc ePhrOn", "Kai Ephron"}, AdaptiveThreshold, false, []string{"zAc ePhrOn", "Kai Ephron"}},
	}

	for i, v := range testCases {
		t.Run("Case "+strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := SelectAll(v.query, v.options, v.threshold, v.caseSensitive)
			if len(got) != len(v.want) {
				t.Errorf("got %#v, wanted %#v", selectionsToStrings(got), v.want)
				return
			}
			for y, s := range got {
				if s.Value != v.want[y] {
					t.Errorf("got %#v, wanted %#v", selectionsToStrings(got), v.want)
					return
				}
			}
		})
	}
}
