package listroles

import (
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestRoleLine(t *testing.T) {
	line := roleLine(&discordgo.Role{ID: 5, Name: "**not bold** `code`", Color: 0x992d22, Permissions: discordgo.PermissionMentionEveryone})

	if !strings.HasPrefix(line, "**\\*\\*not bold\\*\\* \\`code\\`**") {
		t.Fatalf("expected the name to be escaped, got %q", line)
	}

	if !strings.Contains(line, "? members") || !strings.Contains(line, "`5`") || !strings.Contains(line, "#992d22") || !strings.HasSuffix(line, " · ME") {
		t.Fatalf("unexpected line: %q", line)
	}

	if strings.HasSuffix(roleLine(&discordgo.Role{ID: 6, Name: "plain"}), "ME") {
		t.Fatal("roles without mention everyone perms should not be marked")
	}
}
