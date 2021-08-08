package tickets

import (
	"fmt"
	"testing"

	"github.com/jonas747/discordgo"
)

func TestInheritPermissionsFromCategory(t *testing.T) {
	cases := []struct {
		ParentOverwrites []*discordgo.PermissionOverwrite
		InputOverwrites  []*discordgo.PermissionOverwrite
		ExpectedOutput   []*discordgo.PermissionOverwrite
	}{
		{ // 0, basic
			ParentOverwrites: []*discordgo.PermissionOverwrite{},
			InputOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
			},
			ExpectedOutput: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
			},
		},
		{ // 1, basic with role
			ParentOverwrites: []*discordgo.PermissionOverwrite{},
			InputOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
			},
			ExpectedOutput: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
			},
		},
		{ // 2, basic parent check
			ParentOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type: "role",
					ID:   3,
					Deny: discordgo.PermissionReadMessages,
				},
			},
			InputOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
			},
			ExpectedOutput: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
				{
					Type: "role",
					ID:   3,
					Deny: discordgo.PermissionReadMessages,
				},
			},
		},
		{ // 3, allow/deny flip check
			ParentOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type: "role",
					ID:   2,
					Deny: discordgo.PermissionReadMessages,
				},
			},
			InputOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
			},
			ExpectedOutput: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
			},
		},
		{ // 4, multiples
			ParentOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type: "role",
					ID:   2,
					Deny: discordgo.PermissionReadMessages,
				},
				{
					Type:  "role",
					ID:    3,
					Allow: discordgo.PermissionReadMessages,
				},
			},
			InputOverwrites: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
			},
			ExpectedOutput: []*discordgo.PermissionOverwrite{
				{
					Type:  "member",
					ID:    1,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    2,
					Allow: InTicketPerms,
				},
				{
					Type:  "role",
					ID:    3,
					Allow: discordgo.PermissionReadMessages,
				},
			},
		},
	}

	for k, v := range cases {
		t.Run(fmt.Sprintf("Case %d", k), func(t *testing.T) {
			result := applyChannelParentSettingsOverwrites(v.ParentOverwrites, v.InputOverwrites)

			if len(result) != len(v.ExpectedOutput) {
				t.Error("Mismatched lengths")
				return
			}

			for j, r := range result {
				if v.ExpectedOutput[j].Type != r.Type {
					t.Errorf("Overwrite %d: mismatched type, GOT %+v EXPECTED %+v", j, r, v.ExpectedOutput[j])
				}
				if v.ExpectedOutput[j].Allow != r.Allow {
					t.Errorf("Overwrite %d: mismatched allows, GOT %+v EXPECTED %+v", j, r, v.ExpectedOutput[j])
				}
				if v.ExpectedOutput[j].Deny != r.Deny {
					t.Errorf("Overwrite %d: mismatched denies, GOT %+v EXPECTED %+v", j, r, v.ExpectedOutput[j])
				}
				if v.ExpectedOutput[j].ID != r.ID {
					t.Errorf("Overwrite %d: mismatched ID, GOT %+v EXPECTED %+v", j, r, v.ExpectedOutput[j])
				}
			}
		})
	}
}
