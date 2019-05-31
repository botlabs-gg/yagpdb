package viewperms

import (
	"fmt"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "ViewPerms",
	Description: "Shows you or the targets permissions in this channel",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "target", Type: dcmd.UserID, Default: int64(0)},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		target := data.Args[0].Int64()
		if target == 0 {
			target = data.Msg.Author.ID
		}

		perms, err := data.GS.MemberPermissions(true, data.CS.ID, target)
		if err != nil {
			return "Unable to calculate perms (unknown user maybe?)", err
		}

		humanized := common.HumanizePermissions(int64(perms))
		return fmt.Sprintf("`%d`\n%s", perms, strings.Join(humanized, ", ")), nil
	},
}
