package evalcc

import (
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/dcmd/v2"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/discordgo"
	cc "github.com/jonas747/yagpdb/customcommands"
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryTool,
	Name:         "Evalcc",
	Aliases:      []string{"ecc"},
	Description:  "executes custom command code",
	RunInDM:      false,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Expression", Type: dcmd.String},
	},
	RequireDiscordPerms: []int64{discordgo.PermissionManageServer},
	SlashCommandEnabled: false,
	DefaultEnabled:      true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		code := data.Args[0].Str()

		channel := data.GuildData.GS.Channel(true, data.ChannelID)

		if channel == nil {
			return "there was an error", nil
		}

		ctx := templates.NewContext(data.GuildData.GS, channel, data.GuildData.MS)

		ctx.Data["Message"] = data.TraditionalTriggerData.Message

		out, err := ctx.Execute(code)

		if err != nil {
			formatted := cc.FormatError(code, err)
			return "yeah, try again but this time try not to mess up your code smh\n" + formatted, nil
		}

		return out, nil
	},
}


