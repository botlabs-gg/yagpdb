package common

import "testing"

func TestContainsInt64SliceOneOf(t *testing.T) {
	var cases = []struct {
		name   string
		slice  []int64
		search []int64
		want   bool
	}{
		{
			name:   "single match present",
			slice:  []int64{1, 2, 3, 4},
			search: []int64{3},
			want:   true,
		},
		{
			name:   "single not present",
			slice:  []int64{1, 2, 3, 4},
			search: []int64{5},
			want:   false,
		},
		{
			name:   "multiple search values one matches",
			slice:  []int64{10, 20, 30},
			search: []int64{99, 20, 77},
			want:   true,
		},
		{
			name:   "multiple search values none match",
			slice:  []int64{10, 20, 30},
			search: []int64{99, 98, 97},
			want:   false,
		},
		{
			name:   "search has duplicates and value matches",
			slice:  []int64{1, 2, 3},
			search: []int64{2, 2, 2},
			want:   true,
		},
		{
			name:   "slice has duplicates and search matches",
			slice:  []int64{1, 2, 2, 3},
			search: []int64{2},
			want:   true,
		},
		{
			name:   "empty slice non-empty search",
			slice:  []int64{},
			search: []int64{1},
			want:   false,
		},
		{
			name:   "non-empty slice empty search",
			slice:  []int64{1, 2, 3},
			search: []int64{},
			want:   false,
		},
		{
			name:   "both empty",
			slice:  []int64{},
			search: []int64{},
			want:   false,
		},
		{
			name:   "nil slice non-empty search",
			slice:  nil,
			search: []int64{1},
			want:   false,
		},
		{
			name:   "non-empty slice nil search",
			slice:  []int64{1, 2, 3},
			search: nil,
			want:   false,
		},
		{
			name:   "both nil",
			slice:  nil,
			search: nil,
			want:   false,
		},
		{
			name:   "contains zero",
			slice:  []int64{-1, 0, 1},
			search: []int64{0},
			want:   true,
		},
		{
			name:   "contains negative",
			slice:  []int64{-10, -5, 5},
			search: []int64{-5},
			want:   true,
		},
		{
			name:   "search includes negative and positive, negative matches",
			slice:  []int64{100, -200, 300},
			search: []int64{-200, 999},
			want:   true,
		},
		{
			name:   "single element slice match",
			slice:  []int64{42},
			search: []int64{42},
			want:   true,
		},
		{
			name:   "single element slice no match",
			slice:  []int64{42},
			search: []int64{43},
			want:   false,
		},
		{
			name:   "search larger than slice, still matches",
			slice:  []int64{2},
			search: []int64{9, 8, 7, 2, 1},
			want:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := ContainsInt64SliceOneOf(c.slice, c.search)
			if actual != c.want {
				t.Errorf("ContainsInt64SliceOneOf(%v, %v): wanted %v, got %v", c.slice, c.search, c.want, actual)
			}
		})
	}
}

func TestContainsStringSliceFold(t *testing.T) {
	var cases = []struct {
		name   string
		strs   []string
		search string
		want   bool
	}{
		{
			name:   "exact match",
			strs:   []string{"foo", "bar"},
			search: "foo",
			want:   true,
		},
		{
			name:   "case insensitive match",
			strs:   []string{"Foo", "Bar"},
			search: "fOo",
			want:   true,
		},
		{
			name:   "no match",
			strs:   []string{"foo", "bar"},
			search: "baz",
			want:   false,
		},
		{
			name:   "empty slice",
			strs:   []string{},
			search: "foo",
			want:   false,
		},
		{
			name:   "nil slice",
			strs:   nil,
			search: "foo",
			want:   false,
		},
		{
			name:   "empty string search matches empty element",
			strs:   []string{"", "x"},
			search: "",
			want:   true,
		},
		{
			name:   "empty string search does not match when not present",
			strs:   []string{"x", "y"},
			search: "",
			want:   false,
		},
		{
			name:   "whitespace is significant",
			strs:   []string{" foo "},
			search: "foo",
			want:   false,
		},
		{
			name:   "multiple candidates one matches",
			strs:   []string{"alpha", "BETA", "gamma"},
			search: "beta",
			want:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := ContainsStringSliceFold(c.strs, c.search)
			if actual != c.want {
				t.Errorf("ContainsStringSliceFold(%v, %q): wanted %v, got %v", c.strs, c.search, c.want, actual)
			}
		})
	}
}
