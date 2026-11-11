package common

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestRoleColorsText(t *testing.T) {
	cases := []struct {
		role     *discordgo.Role
		expected string
	}{
		{&discordgo.Role{Color: 0x992d22}, "#992d22"},
		{&discordgo.Role{}, "#000000"},
		{&discordgo.Role{Color: 0x992d22, Colors: &discordgo.Colors{PrimaryColor: 0x992d22}}, "#992d22"},
		{&discordgo.Role{Colors: &discordgo.Colors{PrimaryColor: 0x992d22, SecondaryColor: 0x3498db}}, "#992d22/#3498db"},
		{&discordgo.Role{Colors: &discordgo.Colors{PrimaryColor: 0xa9c9ff, SecondaryColor: 0xffbbec, TertiaryColor: 0xffc3a0}}, "#a9c9ff/#ffbbec/#ffc3a0"},
	}

	for _, c := range cases {
		if got := RoleColorsText(c.role); got != c.expected {
			t.Errorf("expected %q, got %q", c.expected, got)
		}
	}
}
