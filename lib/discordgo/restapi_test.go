package discordgo

import (
	"testing"
)

func TestGatewayBot(t *testing.T) {

	if dgBot == nil {
		t.Skip("Skipping, dgBot not set.")
	}
	_, err := dgBot.GatewayBot()

	if err != nil {
		t.Errorf("GatewayBot() returned error: %+v", err)
	}
}
