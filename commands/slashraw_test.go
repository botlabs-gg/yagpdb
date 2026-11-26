package commands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func slashInteraction(name string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: name, Options: options},
	}
}

func opt(name string, typ discordgo.ApplicationCommandOptionType, value any, sub ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: typ, Value: value, Options: sub}
}

func TestSlashCommandRawString(t *testing.T) {
	cases := []struct {
		name        string
		interaction *discordgo.Interaction
		want        string
	}{
		{"no options", slashInteraction("ping"), "/ping"},
		{
			"subcommand with user and role",
			slashInteraction("role", opt("add", discordgo.ApplicationCommandOptionSubCommand, nil,
				opt("user", discordgo.ApplicationCommandOptionUser, "105487308693757952"),
				opt("role", discordgo.ApplicationCommandOptionRole, "204255221017214977"))),
			"/role add user:105487308693757952 role:204255221017214977",
		},
		{
			"subcommand group",
			slashInteraction("settings", opt("prefix", discordgo.ApplicationCommandOptionSubCommandGroup, nil,
				opt("set", discordgo.ApplicationCommandOptionSubCommand, nil,
					opt("value", discordgo.ApplicationCommandOptionString, "!")))),
			"/settings prefix set value:!",
		},
		{
			"numbers and booleans",
			slashInteraction("clean",
				opt("count", discordgo.ApplicationCommandOptionInteger, float64(50)),
				opt("ratio", discordgo.ApplicationCommandOptionNumber, 1.5),
				opt("bots", discordgo.ApplicationCommandOptionBoolean, true)),
			"/clean count:50 ratio:1.5 bots:true",
		},
		{"string with spaces", slashInteraction("say", opt("text", discordgo.ApplicationCommandOptionString, "hello world")), "/say text:hello world"},
		{"nil interaction", nil, "/fallback"},
		{"non command data", &discordgo.Interaction{Type: discordgo.InteractionMessageComponent}, "/fallback"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := slashCommandRawString(c.interaction, "fallback"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
