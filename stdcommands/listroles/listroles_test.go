package listroles

import (
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestRoleLine(t *testing.T) {
	line := roleLine(&discordgo.Role{ID: 5, Name: "@everyone", Color: 0x992d22, Permissions: discordgo.PermissionMentionEveryone})
	if line != "`@everyone                : 5                   #992d22  Members:?      ME: true`\n" {
		t.Fatalf("unexpected line: %q", line)
	}

	if line := roleLine(&discordgo.Role{ID: 6, Name: "tick`s"}); strings.Count(line, "`") != 2 {
		t.Fatalf("expected backticks in names to not end the code span, got %q", line)
	}
}
