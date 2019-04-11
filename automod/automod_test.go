package automod

import (
	"strconv"
	"testing"
)

func TestPrepareMessageForWordCheck(t *testing.T) {
	cases := []struct {
		input  string
		output string
	}{
		{input: "!play", output: " play play !play"},
		{input: "wew", output: "wew"},
		{input: "we*w", output: "we w wew we*w"},
		{input: "we**w", output: "we  w wew we**w"},
	}

	for i, c := range cases {
		t.Run("#"+strconv.Itoa(i), func(st *testing.T) {
			result := PrepareMessageForWordCheck(c.input)
			if result != c.output {
				st.Errorf("got: %q, expected: %q", result, c.output)
			}
		})
	}
}
