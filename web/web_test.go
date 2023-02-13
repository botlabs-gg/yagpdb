package web

import (
	"fmt"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func createUserGuild(connected bool, owner bool, manageServer bool) *common.GuildWithConnected {
	perms := int64(0)
	if manageServer {
		perms = discordgo.PermissionManageServer
	}

	if owner {
		perms |= discordgo.PermissionManageServer | discordgo.PermissionAll
	}

	return &common.GuildWithConnected{
		Connected: connected,
		UserGuild: &discordgo.UserGuild{
			Owner:       owner,
			Permissions: perms,
		},
	}
}

func TestHasAccesstoGuildSettings(t *testing.T) {

	type TestCase struct {
		Name     string
		Conf     *models.CoreConfig
		GWC      *common.GuildWithConnected
		Roles    []int64
		IsMember bool
		ReadOnly bool

		ShouldHaveAccess bool
	}

	testCases := []*TestCase{

		//////////////////////////
		// default settings tests
		/////////////////////////

		// default settings, non member access
		{
			Name:     "default settings non member access (ro)",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name:     "default settings non member access",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// default settings normal member access
		{
			Name:     "default settings normal normal member access (ro)",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name:     "default settings normal user access",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// default settings admin user access
		{
			Name:     "default settings admin user access (ro)",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name:     "default settings admin user access",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		// default settings owner user access
		{
			Name:     "default settings owner user access (ro)",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, true, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name:     "default settings owner user access",
			Conf:     &models.CoreConfig{},
			GWC:      createUserGuild(true, true, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		////////////////////////////////////
		//   AllowNonMembersROAccess tests
		////////////////////////////////////

		// all users ro - normal user access
		{
			Name: "all users ro-normal user access (ro)",
			Conf: &models.CoreConfig{
				AllowNonMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "all users ro-normal user access",
			Conf: &models.CoreConfig{
				AllowNonMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},
		// all users ro - member access
		{
			Name: "all users ro-member access (ro)",
			Conf: &models.CoreConfig{
				AllowNonMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "all users ro-member access",
			Conf: &models.CoreConfig{
				AllowNonMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},
		// all users ro - admin access
		{
			Name: "all users ro-admin access (ro)",
			Conf: &models.CoreConfig{
				AllowNonMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "all users ro-admin access",
			Conf: &models.CoreConfig{
				AllowNonMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		////////////////////////////////////
		//   AllMembersRO tests
		////////////////////////////////////

		// all members ro - normal user access
		{
			Name: "all members ro-normal user access (ro)",
			Conf: &models.CoreConfig{
				AllowAllMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name: "all members ro-normal user access",
			Conf: &models.CoreConfig{
				AllowAllMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},
		// all members ro - member access
		{
			Name: "all members ro-member access (ro)",
			Conf: &models.CoreConfig{
				AllowAllMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "all members ro-member access",
			Conf: &models.CoreConfig{
				AllowAllMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},
		// all members ro - admin access
		{
			Name: "all members ro-admin access (ro)",
			Conf: &models.CoreConfig{
				AllowAllMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "all members ro-admin access",
			Conf: &models.CoreConfig{
				AllowAllMembersReadOnly: true,
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		////////////////////////////////////
		//   Read only roles
		////////////////////////////////////

		// ro roles - normal user access
		{
			Name: "ro roles-normal user access (ro)",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name: "ro roles-normal user access",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// ro roles - member no roles
		{
			Name: "ro roles-member no roles (ro)",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name: "ro roles-member no roles",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// ro roles - member access one role
		{
			Name: "ro roles-member access one role (ro)",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5},
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "ro roles-member access one role",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5},
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// ro roles - member access other role
		{
			Name: "ro roles-member access other role (ro)",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{6},
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "ro roles-member access other role",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{6},
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// ro roles - member access both roles
		{
			Name: "ro roles - member access both roles (ro)",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5, 6},
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "ro roles - member access both roles",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5, 6},
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// ro roles - admin access
		{
			Name: "ro roles-admin access (ro)",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "ro roles-admin access",
			Conf: &models.CoreConfig{
				AllowedReadOnlyRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		////////////////////////////////////
		//   Write roles
		////////////////////////////////////

		// write roles - normal user access
		{
			Name: "write roles-normal user access (ro)",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name: "write roles-normal user access",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: false,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// write roles - member no roles
		{
			Name: "write roles-member no roles (ro)",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: false,
		},
		{
			Name: "write roles-member no roles",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: false,
		},

		// write roles - member access one role
		{
			Name: "write roles-member access one role (ro)",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5},
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "write roles-member access one role",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5},
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		// write roles - member access other role
		{
			Name: "write roles-member access other role (ro)",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{6},
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "write roles-member access other role",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{6},
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		// write roles - member access both roles
		{
			Name: "write roles - member access both roles (ro)",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5, 6},
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "write roles - member access both roles",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, false),
			Roles:    []int64{5, 6},
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},

		// write roles - admin access
		{
			Name: "write roles-admin access (ro)",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: true,

			ShouldHaveAccess: true,
		},
		{
			Name: "write roles-admin access",
			Conf: &models.CoreConfig{
				AllowedWriteRoles: []int64{5, 6},
			},
			GWC:      createUserGuild(true, false, true),
			Roles:    nil,
			IsMember: true,
			ReadOnly: false,

			ShouldHaveAccess: true,
		},
	}

	for i, v := range testCases {
		t.Run(fmt.Sprintf("Case #%d_%s", i, v.Name), func(it *testing.T) {
			userID := int64(0)
			if v.IsMember {
				userID = 10
			}

			result := HasAccesstoGuildSettings(userID, v.GWC, v.Conf, StaticRoleProvider(v.Roles), !v.ReadOnly)
			if result != v.ShouldHaveAccess {
				it.Errorf("incorrect result, got %t, wanted: %t", result, v.ShouldHaveAccess)
			}
		})
	}

}
