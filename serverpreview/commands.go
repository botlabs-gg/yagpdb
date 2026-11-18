package serverpreview

import (
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, suspendCommand(true), suspendCommand(false))
}

func suspendCommand(suspend bool) *commands.YAGCommand {
	name, description, reply := "suspendpreview", "Disables a server's preview and stops its admins from turning it back on. Bot Owner Only", "Suspended the server preview of "
	if !suspend {
		name, description, reply = "unsuspendpreview", "Lifts a server preview suspension. Bot Owner Only", "Lifted the server preview suspension of "
	}

	return &commands.YAGCommand{
		CmdCategory:          commands.CategoryDebug,
		HideFromCommandsPage: true,
		Name:                 name,
		Description:          description,
		RequiredArgs:         1,
		Arguments: []*dcmd.ArgDef{
			{Name: "server", Type: dcmd.BigInt},
		},
		RunFunc: util.RequireOwner(func(data *dcmd.Data) (any, error) {
			if err := SetSuspended(data.Args[0].Int64(), suspend); err != nil {
				return nil, err
			}
			return reply + data.Args[0].Str(), nil
		}),
	}
}
