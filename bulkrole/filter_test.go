package bulkrole

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestFilterMemberRoleFilters(t *testing.T) {
	const (
		roleA int64 = 1
		roleB int64 = 2
		roleC int64 = 3
		roleD int64 = 4
		other int64 = 99
	)
	filterRoles := []int64{roleA, roleB, roleC, roleD}

	member := func(roles ...int64) *discordgo.Member {
		return &discordgo.Member{Roles: roles, User: &discordgo.User{ID: 1234}}
	}

	tests := []struct {
		name       string
		filterType string
		requireAll bool
		member     *discordgo.Member
		want       bool
	}{
		{"missing, toggle on, holds none of them", "missing_roles", true, member(other), true},
		{"missing, toggle on, no roles at all", "missing_roles", true, member(), true},
		{"missing, toggle on, holds one of them", "missing_roles", true, member(other, roleB), false},
		{"missing, toggle on, holds all four", "missing_roles", true, member(roleA, roleB, roleC, roleD), false},

		{"missing, toggle off, holds none of them", "missing_roles", false, member(other), true},
		{"missing, toggle off, holds one of them", "missing_roles", false, member(other, roleB), true},
		{"missing, toggle off, holds three of them", "missing_roles", false, member(roleA, roleB, roleC), true},
		{"missing, toggle off, holds all four", "missing_roles", false, member(roleA, roleB, roleC, roleD), false},

		{"has, toggle on, holds all four", "has_roles", true, member(roleA, roleB, roleC, roleD), true},
		{"has, toggle on, holds three of them", "has_roles", true, member(roleA, roleB, roleC), false},
		{"has, toggle off, holds one of them", "has_roles", false, member(roleB), true},
		{"has, toggle off, holds none of them", "has_roles", false, member(other), false},

		{"missing, no filter roles selected", "missing_roles", true, member(other), false},
		{"has, no filter roles selected", "has_roles", true, member(roleA), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			roles := filterRoles
			if tc.name == "missing, no filter roles selected" || tc.name == "has, no filter roles selected" {
				roles = nil
			}

			config := &BulkRoleConfig{
				FilterType:       tc.filterType,
				FilterRoleIDs:    roles,
				FilterRequireAll: tc.requireAll,
			}

			if got := config.filterMember(tc.member); got != tc.want {
				t.Errorf("filterMember() = %v, want %v", got, tc.want)
			}
		})
	}
}
