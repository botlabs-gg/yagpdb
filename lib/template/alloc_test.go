package template

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func allocTestTemplate(t *testing.T, src string, maxAlloc int) *Template {
	t.Helper()
	funcs := FuncMap{
		"print": fmt.Sprint,
		"big":   func() string { return strings.Repeat("0", 999990) },
		"seq": func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i
			}
			return out
		},
		"trim":   strings.TrimSpace,
		"upper":  strings.ToUpper,
		"substr": func(s string, from, to int) string { return s[from:to] },
	}

	tmpl, err := New("alloc").Funcs(funcs).Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return tmpl.MaxAllocBytes(maxAlloc)
}

func TestMaxAllocBytesStopsStringChurn(t *testing.T) {
	tmpl := allocTestTemplate(t, `{{$big := big}}{{range seq 10000}}{{$x := print $big .}}{{end}}`, 50<<20)

	allocated, err := tmpl.ExecuteCountingAlloc(io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "exceeded max memory allocation") {
		t.Fatalf("expected the allocation cap to stop the loop, got err=%v", err)
	}
	if allocated > 52<<20 {
		t.Fatalf("expected execution to stop just past the 50MB cap, allocated %dMB", allocated>>20)
	}
}

func TestMaxAllocBytesIgnoresValuesThatAreNotNewAllocations(t *testing.T) {
	src := `{{$big := print " " big " "}}{{$list := seq 100}}` +
		`{{range seq 1000}}{{$a := index $list 1}}{{$b := substr $big 1 500000}}{{$c := trim $big}}{{$d := upper $big}}{{end}}`
	tmpl := allocTestTemplate(t, src, 10<<20)

	allocated, err := tmpl.ExecuteCountingAlloc(io.Discard, nil)
	if err != nil {
		t.Fatalf("indexing and substrings must not be charged, got %v", err)
	}
	if allocated > 2<<20 {
		t.Fatalf("expected only the initial ~1MB print to be charged, got %dMB", allocated>>20)
	}
}

func TestMaxAllocBytesChargesNestedTemplatesToTheRoot(t *testing.T) {
	src := `{{define "churn"}}{{$big := big}}{{range seq 20}}{{$x := print $big .}}{{end}}{{end}}` +
		`{{range seq 10}}{{template "churn"}}{{end}}`
	tmpl := allocTestTemplate(t, src, 100<<20)

	_, err := tmpl.ExecuteCountingAlloc(io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "exceeded max memory allocation") {
		t.Fatalf("expected nested template allocations to count against the root budget, got err=%v", err)
	}
}

func TestMaxAllocBytesZeroDisablesTheCap(t *testing.T) {
	tmpl := allocTestTemplate(t, `{{$big := big}}{{range seq 20}}{{$x := print $big .}}{{end}}`, 0)

	if _, err := tmpl.ExecuteCountingAlloc(io.Discard, nil); err != nil {
		t.Fatalf("expected no cap when MaxAllocBytes is unset, got %v", err)
	}
}
