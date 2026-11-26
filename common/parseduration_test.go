package common

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"10", 10 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"1day3h", 27 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"1mo", 30 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"292y", 292 * 365 * 24 * time.Hour},
	}

	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if err != nil {
			t.Errorf("ParseDuration(%q) returned error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseDurationOverflow(t *testing.T) {
	inputs := []string{
		"9999999999999999y",
		"300y",
		"9223372036854775807s",
		"99999999999999999999s",
		"200y200y",
		"292y1y",
	}

	for _, in := range inputs {
		got, err := ParseDuration(in)
		if err == nil {
			t.Errorf("ParseDuration(%q) = %v, want error", in, got)
		}
	}
}
