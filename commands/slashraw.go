package commands

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// slashCommandRawString renders a slash command the way Discord shows it, such as
// "/role add user:123 role:456", so executed command logs keep the arguments.
func slashCommandRawString(interaction *discordgo.Interaction, fallbackName string) string {
	if interaction == nil {
		return "/" + fallbackName
	}
	data, ok := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if !ok {
		return "/" + fallbackName
	}

	var b strings.Builder
	b.WriteString("/")
	b.WriteString(data.Name)
	writeSlashOptions(&b, data.Options)
	return b.String()
}

func writeSlashOptions(b *strings.Builder, options []*discordgo.ApplicationCommandInteractionDataOption) {
	for _, opt := range options {
		if opt == nil {
			continue
		}

		b.WriteString(" ")
		b.WriteString(opt.Name)

		switch opt.Type {
		case discordgo.ApplicationCommandOptionSubCommand, discordgo.ApplicationCommandOptionSubCommandGroup:
			writeSlashOptions(b, opt.Options)
		default:
			b.WriteString(":")
			b.WriteString(slashOptionValue(opt.Value))
		}
	}
}

func slashOptionValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprint(t)
	default:
		return fmt.Sprint(t)
	}
}
