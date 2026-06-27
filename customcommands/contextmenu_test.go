package customcommands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestContextMenuNameRegex(t *testing.T) {
	valid := []string{"Report", "Report Message", "flag-user", "abc 123", "ünïcode name"}
	invalid := []string{"", "bad.name", "way too long name that exceeds the limit!!", "no_emoji_😀"}

	for _, v := range valid {
		if !contextMenuNameRegex.MatchString(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	for _, v := range invalid {
		if contextMenuNameRegex.MatchString(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestToDBModelContextMenu(t *testing.T) {
	cc := &CustomCommand{
		TriggerTypeForm: "user_context_menu",
		TriggerType:     CommandTriggerUserContextMenu,
		Trigger:         "  Report User  ", // case + spaces preserved, surrounding space trimmed
		Responses:       []string{"hi"},
	}

	db := cc.ToDBModel()
	if db.TriggerType != int(CommandTriggerUserContextMenu) {
		t.Errorf("unexpected trigger type %d", db.TriggerType)
	}
	if db.TextTrigger != "Report User" {
		t.Errorf("expected trimmed, case-preserved name %q, got %q", "Report User", db.TextTrigger)
	}
	if db.SlashCommandOptions.Valid {
		t.Error("context menu commands should not store slash command options")
	}
}

func TestBuildContextMenuCommandRequest(t *testing.T) {
	user := &models.CustomCommand{TextTrigger: "Report User", TriggerType: int(CommandTriggerUserContextMenu)}
	req := buildContextMenuCommandRequest(user)
	if req.Name != "Report User" {
		t.Errorf("unexpected name %q", req.Name)
	}
	if req.Type != discordgo.UserApplicationCommand {
		t.Errorf("expected USER command type, got %v", req.Type)
	}
	if req.Description != "" || len(req.Options) != 0 {
		t.Errorf("context menu commands must have no description/options: %+v", req)
	}

	msg := &models.CustomCommand{TextTrigger: "Report Message", TriggerType: int(CommandTriggerMessageContextMenu)}
	if got := buildContextMenuCommandRequest(msg); got.Type != discordgo.MessageApplicationCommand {
		t.Errorf("expected MESSAGE command type, got %v", got.Type)
	}
}

func TestIsContextMenuTrigger(t *testing.T) {
	if !IsContextMenuTrigger(CommandTriggerUserContextMenu) || !IsContextMenuTrigger(CommandTriggerMessageContextMenu) {
		t.Error("expected both context menu types to report true")
	}
	if IsContextMenuTrigger(CommandTriggerSlash) || IsContextMenuTrigger(CommandTriggerCommand) {
		t.Error("non context menu types must report false")
	}
}
