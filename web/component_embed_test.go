package web

import (
	"encoding/json"
	"testing"
)

func TestLandingComponentEmbedFitsDiscordLimits(t *testing.T) {
	encoded, err := landingComponentEmbed().Encode()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("landing component embed is %d bytes", len(encoded))

	var decoded struct {
		Component map[string]any `json:"component"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Component["type"] != float64(17) {
		t.Errorf("top level component must be a container, got %v", decoded.Component["type"])
	}
}
