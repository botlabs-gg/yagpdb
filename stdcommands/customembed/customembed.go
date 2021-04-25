package customembed

import (
	"encoding/json"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryFun,
	Name:            "CustomEmbed",
	Aliases:         []string{"ce"},
	Description:     "Creates an embed from what you give it in json form: https://docs.yagpdb.xyz/others/custom-embeds",
	LongDescription: "Example: `-ce {\"title\": \"hello\", \"description\": \"wew\"}`",
	RequiredArgs:    1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Json", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var parsed *discordgo.MessageEmbed
		err := json.Unmarshal([]byte(data.Args[0].Str()), &parsed)
		if err != nil {
			return "Failed parsing json: " + err.Error(), err
		}	
		// fallback for missing embed fields
		if parsed.Description == "" {
			parsed.Description = "\u200b"
		}
		if parsed.Title == "" && parsed.URL != "" {
			return "Title is required for URL", nil
		}
		if parsed.Author.Name == "" && parsed.Author.IconURL != "" {
			parsed.Author.Name = "\u200b"
		} else if parsed.Author.Name == "" && parsed.Author.URL != "" {
			return "Author Name is required for Author URL", nil
		}
		if parsed.Footer.Name == "" && parsed.Footer.IconURL != "" {
			parsed.Footer.Name = "\u200b"
		}
		return parsed, nil
	},
}
