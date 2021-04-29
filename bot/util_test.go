package bot

import (
	"fmt"
	"testing"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v2"
)

func TestMemberHighestRole(t *testing.T) {
	gs := &dstate.GuildState{
		Guild: &discordgo.Guild{
			Roles: []*discordgo.Role{
				&discordgo.Role{ID: 10, Position: 10},
				&discordgo.Role{ID: 5, Position: 5},
				&discordgo.Role{ID: 100, Position: 1},
				&discordgo.Role{ID: 102, Position: 1},
			},
		},
	}

	cases := []struct {
		Roles   []int64
		Highest int64
	}{
		{Roles: []int64{100, 5, 10}, Highest: 10},
		{Roles: []int64{102, 100}, Highest: 100},
		{Roles: []int64{5, 102}, Highest: 5},
	}

	for i, v := range cases {
		t.Run(fmt.Sprintf("case #%d", i), func(t *testing.T) {
			ms := &dstate.MemberState{
				Roles: v.Roles,
			}

			result := MemberHighestRole(gs, ms)
			if result.ID != v.Highest {
				t.Errorf("incorrect result, got %d, expected %d", result.ID, v.Highest)
			}
		})
	}
}

func TestIsMemberAbove(t *testing.T) {
	gs := &dstate.GuildState{
		Guild: &discordgo.Guild{
			OwnerID: 99,
			Roles: []*discordgo.Role{
				&discordgo.Role{ID: 10, Position: 10},
				&discordgo.Role{ID: 5, Position: 5},
				&discordgo.Role{ID: 100, Position: 1},
				&discordgo.Role{ID: 102, Position: 1},
			},
		},
	}

	cases := []struct {
		M1 []int64
		M2 []int64

		Above bool
	}{
		{M1: []int64{100, 5}, M2: []int64{10, 100}, Above: false},
		{M1: []int64{100, 5, 10}, M2: []int64{10, 100}, Above: false},
		{M1: []int64{100, 5, 10}, M2: []int64{100}, Above: true},
		{M1: []int64{100, 102}, M2: []int64{102}, Above: true},
		{M1: []int64{100}, M2: []int64{100}, Above: false},
	}

	for i, v := range cases {
		t.Run(fmt.Sprintf("case #%d", i), func(t *testing.T) {
			ms1 := &dstate.MemberState{
				Roles: v.M1,
			}

			ms2 := &dstate.MemberState{
				Roles: v.M2,
			}

			result := IsMemberAbove(gs, ms1, ms2)
			if result != v.Above {
				t.Errorf("incorrect result, got %t, expected %t", result, v.Above)
			}
		})
	}
}
