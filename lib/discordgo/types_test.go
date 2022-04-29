package discordgo

import (
	"testing"
	"time"
)

func TestTimestampParse(t *testing.T) {
	ts, err := Timestamp("2016-03-24T23:15:59.605000+00:00").Parse()
	if err != nil {
		t.Fatal(err)
	}
	if ts.Year() != 2016 || ts.Month() != time.March || ts.Day() != 24 {
		t.Error("Incorrect date")
	}
	if ts.Hour() != 23 || ts.Minute() != 15 || ts.Second() != 59 {
		t.Error("Incorrect time")
	}

	_, offset := ts.Zone()
	if offset != 0 {
		t.Error("Incorrect timezone")
	}
}

// func TestEmojiNameUnqualify(t *testing.T) {
// 	cases := []struct {
// 		have string
// 		want string
// 	}{
// 		{"⚔️", "⚔"},
// 		{"⚔", "⚔"},
// 		{":crossed_swords:", "⚔"},
// 		{"1o57:442605016813928449", "1o57:442605016813928449"},
// 	}

// 	for _, c := range cases {
// 		emoji := EmojiName{c.have}
// 		if emoji.String() != c.want {
// 			t.Errorf("Failed to strip emoji qualifier: '%s' -> '%s' not '%s'",
// 				c.have, emoji.String(), c.want)
// 		}
// 	}
// }
