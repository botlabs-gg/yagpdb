package discordgo

import (
	"encoding/json"
	"testing"
)

func TestModalSubmitTextInputValue(t *testing.T) {
	payloads := map[string]string{
		"label":       `{"custom_id":"m","components":[{"type":18,"component":{"type":4,"custom_id":"reason","value":"from label"}}]}`,
		"actions row": `{"custom_id":"m","components":[{"type":1,"components":[{"type":4,"custom_id":"reason","value":"from actions row"}]}]}`,
	}

	for name, payload := range payloads {
		var data ModalSubmitInteractionData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		value, ok := data.TextInputValue("reason")
		if !ok || value != "from "+name {
			t.Fatalf("%s: got %q, %t", name, value, ok)
		}

		if _, ok := data.TextInputValue("missing"); ok {
			t.Fatalf("%s: found a text input that isn't there", name)
		}
	}
}
