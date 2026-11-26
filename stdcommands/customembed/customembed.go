package customembed

import (
	"encoding/json"
	"errors"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/token"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryTool,
	Name:                "CustomEmbed",
	Aliases:             []string{"ce"},
	Description:         "Creates an embed from what you give it in json form: https://help.yagpdb.xyz/docs/reference/custom-embeds/",
	LongDescription:     "Example: `/customembed {\"title\": \"hello\", \"description\": \"wew\"}`",
	RequiredArgs:        1,
	RequireDiscordPerms: []int64{discordgo.PermissionManageMessages},
	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	Arguments: []*dcmd.ArgDef{
		{Name: "Json", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (any, error) {
		j := common.ParseCodeblock(data.Args[0].Str())
		parsed, err := parseEmbed(j)
		if err != nil {
			return "Failed parsing the embed JSON.", nil
		}

		if discordgo.IsEmbedEmpty(parsed) {
			return "Cannot send an empty embed", nil
		}

		return parsed, nil
	},
}

// Larger than any embed discord accepts, while keeping goccy/go-yaml away from big inputs.
const maxYAMLSize = 6000

var errUnsupportedYAML = errors.New("unsupported yaml input")

func parseEmbed(input string) (*discordgo.MessageEmbed, error) {
	var parsed discordgo.MessageEmbed
	if err := json.Unmarshal([]byte(input), &parsed); err == nil {
		return &parsed, nil
	}

	if len(input) > maxYAMLSize {
		return nil, errUnsupportedYAML
	}

	for _, tk := range lexer.Tokenize(input) {
		if tk.Type == token.AnchorType || tk.Type == token.AliasType {
			return nil, errUnsupportedYAML
		}
	}

	parsed = discordgo.MessageEmbed{}
	if err := yaml.Unmarshal([]byte(input), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
