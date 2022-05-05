package bot

import (
	"fmt"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func TestMemberHighestRole(t *testing.T) {
	gs := &dstate.GuildSet{
		GuildState: dstate.GuildState{},
		Roles: []discordgo.Role{
			{ID: 10, Position: 10},
			{ID: 5, Position: 5},
			{ID: 100, Position: 1},
			{ID: 102, Position: 1},
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
				Member: &dstate.MemberFields{
					Roles: v.Roles,
				},
			}

			result := MemberHighestRole(gs, ms)
			if result.ID != v.Highest {
				t.Errorf("incorrect result, got %d, expected %d", result.ID, v.Highest)
			}
		})
	}
}

func TestIsMemberAbove(t *testing.T) {
	gs := &dstate.GuildSet{
		GuildState: dstate.GuildState{
			OwnerID: 99,
		},
		Roles: []discordgo.Role{
			{ID: 10, Position: 10},
			{ID: 5, Position: 5},
			{ID: 100, Position: 1},
			{ID: 102, Position: 1},
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
				Member: &dstate.MemberFields{
					Roles: v.M1,
				},
			}

			ms2 := &dstate.MemberState{
				Member: &dstate.MemberFields{
					Roles: v.M2,
				},
			}

			result := IsMemberAbove(gs, ms1, ms2)
			if result != v.Above {
				t.Errorf("incorrect result, got %t, expected %t", result, v.Above)
			}
		})
	}
}
