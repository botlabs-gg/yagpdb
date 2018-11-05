package main

import (
	"bytes"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"os"
	"strings"
)

func GenCommandsDocs() {
	// func GenerateHelp(d *Data, container *Container, formatter HelpFormatter) (embeds []*discordgo.MessageEmbed) {

	sets := dcmd.SortCommands(commands.CommandSystem.Root, commands.CommandSystem.Root)

	var out bytes.Buffer

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

			if args != "" {
				out.WriteString("```\n" + args + "\n```\n")
			}

			if switches != "" {
				out.WriteString("```\n" + switches + "\n```\n")
			}

			out.WriteString(desc)
			out.WriteString("\n\n")

		}

	}

	os.Stdout.Write(out.Bytes())

	return
}
