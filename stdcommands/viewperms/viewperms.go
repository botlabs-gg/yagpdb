package viewperms

import (
	"fmt"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "ViewPerms",
	Description: "Shows you or the targets permissions in this channel",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "target", Type: &commands.MemberArg{}},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		member := commands.ContextMS(data.Context())

		if data.Args[0].Value != nil {
			member = data.Args[0].Value.(*dstate.MemberState)
		} else {
			member, _ = bot.GetMember(member.Guild.ID, member.ID)
		}

		perms, err := member.Guild.MemberPermissionsMS(true, data.CS.ID, member)
		if err != nil {
			return "Unable to calculate perms (unknown user maybe?)", err
		}

		humanized := common.HumanizePermissions(int64(perms))
		return fmt.Sprintf("`%d`\n%s", perms, strings.Join(humanized, ", ")), nil
	},
}
