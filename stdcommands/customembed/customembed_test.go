package customembed

import (
	"strings"
	"testing"
)

func TestParseEmbed(t *testing.T) {
	valid := map[string]string{
		`{"title": "hello", "description": "wew **bold** & more"}`: "hello",
		"title: hello\ndescription: wew":                           "hello",
		"{title: hello, color: 123}":                               "hello",
	}
	for in, wantTitle := range valid {
		e, err := parseEmbed(in)
		if err != nil {
			t.Errorf("parseEmbed(%q) returned error: %v", in, err)
			continue
		}
		if e.Title != wantTitle {
			t.Errorf("parseEmbed(%q) title = %q, want %q", in, e.Title, wantTitle)
		}
	}

	invalid := []string{
		"title: &a hello\ndescription: *a",
		"title: x\ndescription: " + strings.Repeat("a", maxYAMLSize),
		"{not valid",
	}
	for _, in := range invalid {
		if _, err := parseEmbed(in); err == nil {
			t.Errorf("parseEmbed(%q) expected an error", in)
		}
	}
}
