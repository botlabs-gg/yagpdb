package common

import "testing"

func TestEscapeSpecialMentions(t *testing.T) {
	cases := [][]string{
		//      [0]Test input        [1]Expected output
		[]string{"@everyone", "@" + zeroWidthSpace + "everyone"},
		[]string{"@here", "@" + zeroWidthSpace + "here"},
		[]string{"<@&245230583637213184>", "<@" + zeroWidthSpace + "&245230583637213184>"},
		[]string{"<@&245230583637213184> hello <@&242230583637213184>", "<@" + zeroWidthSpace + "&245230583637213184> hello <@" + zeroWidthSpace + "&242230583637213184>"},
	}

	for _, c := range cases {
		t.Run("Case "+c[0], func(innerT *testing.T) {
			result := EscapeSpecialMentions(c[0])
			if result != c[1] {
				innerT.Errorf("Incorrect result, Got: %q, Expected: %q", result, c[1])
			}
		})
	}
}
