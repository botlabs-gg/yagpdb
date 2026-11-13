package discordgo

import (
	"encoding/json"
	"testing"
)

func TestInteractionChannelDecode(t *testing.T) {
	var i Interaction
	payload := `{"type":2,"data":{"id":"1","name":"ping","type":1},"channel_id":"5","channel":{"id":"5","name":"hidden","type":0},"app_permissions":"2048"}`
	if err := json.Unmarshal([]byte(payload), &i); err != nil {
		t.Fatal(err)
	}

	if i.Channel == nil || i.Channel.ID != 5 || i.Channel.Name != "hidden" || i.AppPermissions != PermissionSendMessages {
		t.Fatalf("unexpected interaction channel data: %+v %d", i.Channel, i.AppPermissions)
	}
}
