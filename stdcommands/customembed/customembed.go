package customembed

import (
	"encoding/json"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"gopkg.in/yaml.v3"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryTool,
	Name:                "CustomEmbed",
	Aliases:             []string{"ce"},
	Description:         "Creates an embed from what you give it in json form: https://docs.yagpdb.xyz/others/custom-embeds",
	LongDescription:     "Example: `-ce {\"title\": \"hello\", \"description\": \"wew\"}`",
	RequiredArgs:        1,
	RequireDiscordPerms: []int64{discordgo.PermissionManageMessages},
	Arguments: []*dcmd.ArgDef{
		{Name: "Json", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		j := common.ParseCodeblock(data.Args[0].Str())
		var parsed *discordgo.MessageEmbed

		// attempt to parse as YAML first.
		// We don't care about the error here, as we're going to try parsing it as JSON anyway.
		err := yaml.Unmarshal([]byte(j), &parsed)
		if err != nil {
			// Maybe it is JSON instead?
			err = json.Unmarshal([]byte(j), &parsed)
			if err != nil {
				return "Failed parsing as YAML or JSON", err
			}
		}

		if discordgo.IsEmbedEmpty(parsed) {
			return "Cannot send an empty embed", nil
		}
		return parsed, nil
	},
}
