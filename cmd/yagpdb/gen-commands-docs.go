package main

import (
	"bytes"
	"os"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

func GenCommandsDocs() {
	// func GenerateHelp(d *Data, container *Container, formatter HelpFormatter) (embeds []*discordgo.MessageEmbed) {

	sets := dcmd.SortCommands(commands.CommandSystem.Root, commands.CommandSystem.Root)

	var out bytes.Buffer
	out.WriteString("## Legend\n\n")
	out.WriteString("`<required arg>` `[optional arg]`\n\n")
	out.WriteString("Text arguments containing multiple words needs be to put in quotes (\"arg here\") or code ticks (`arg here`) if it's not the last argument and there's more than 1 text argument.\n\n")
	out.WriteString("For example with the poll command if you want the question to have multiple words: `-poll \"whats your favorite color\" red blue green2`\n\n")

	stdHelpFmt := &dcmd.StdHelpFormatter{}
	mockCmdData := &dcmd.Data{}

	for _, set := range sets {

		out.WriteString("## " + set.Name() + " " + set.Emoji() + "\n\n")

		for _, entry := range set.Commands {
			// get the main name
			nameStr := entry.Container.FullName(false)
			if nameStr != "" {
				nameStr += " "
			}
			nameStr += entry.Cmd.Trigger.Names[0]

			// then aliases
			aliases := strings.Join(entry.Cmd.Trigger.Names[1:], "/")

			// arguments and switches
			args := stdHelpFmt.ArgDefs(entry.Cmd, mockCmdData)
			switches := stdHelpFmt.Switches(entry.Cmd.Command)

			// grab the description
			desc := ""
			if cast, ok := entry.Cmd.Command.(dcmd.CmdWithDescriptions); ok {
				short, long := cast.Descriptions(mockCmdData)
				if long != "" {
					desc = long
				} else if short != "" {
					desc = short
				} else {
					desc = "No description for this command"
				}
			}

			out.WriteString("### " + nameStr + "\n\n")
			if aliases != "" {
				out.WriteString("**Aliases:** " + aliases + "\n\n")
			}

			out.WriteString(desc)
			out.WriteString("\n\n")

			out.WriteString("**Usage:**\n")
			out.WriteString("```\n" + args + "\n```\n")

			if switches != "" {
				out.WriteString("```\n" + switches + "\n```\n")
			}

			out.WriteString("\n")
		}

	}

	os.Stdout.Write(out.Bytes())

	return
}
