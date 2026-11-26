package safebrowsing

import (
	"slices"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
)

func serverMatches(input string) []string {
	return common.LinkRegex.FindAllString(confusables.NormalizeQueryEncodedText(input), -1)
}

func TestRemoteBatchesMatchPerStringLookups(t *testing.T) {
	inputs := []string{
		"no links here",
		"",
		"see https://example.com/path and http://foo.bar",
		"encoded https%3A//enc.example.com/x",
		"bad escape %zz https://invalid-escape.example",
		"https://another.example?q=1",
		"plain text again",
		"https://last.example",
		"%E2%82 https://partial-utf8.example",
		"\u0301https://combining.example",
	}

	var want []string
	for _, in := range inputs {
		want = append(want, serverMatches(in)...)
	}

	batches := remoteBatches(inputs)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d: %q", len(batches), batches)
	}

	var got []string
	for _, b := range batches {
		for _, in := range b {
			if len(serverMatches(in)) == 0 {
				t.Errorf("batched a string without links: %q", in)
			}
		}
		got = append(got, serverMatches(strings.Join(b, "\n"))...)
	}

	if !slices.Equal(want, got) {
		t.Fatalf("batched matches differ\nwant %q\ngot  %q", want, got)
	}
}

func TestRemoteBatchesSkipsLinkless(t *testing.T) {
	if b := remoteBatches([]string{"hello", "world", ""}); len(b) != 0 {
		t.Fatalf("expected no batches, got %q", b)
	}
}
