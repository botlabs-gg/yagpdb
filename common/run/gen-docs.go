package run

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common/config"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

func GenCommandsDocs() {
	// func GenerateHelp(d *Data, container *Container, formatter HelpFormatter) (embeds []*discordgo.MessageEmbed) {

	sets := dcmd.SortCommands(commands.CommandSystem.Root, commands.CommandSystem.Root)

	var out bytes.Buffer
	out.WriteString("## Legend\n\n")
	out.WriteString("`<required arg>` `[optional arg]`\n\n")
	out.WriteString("Text arguments containing multiple words needs be to put in quotes (\"arg here\") or code ticks (`arg here`) if it's not the last argument and there's more than 1 text argument.\n\n")
	out.WriteString("For example with the poll command if you want the question to have multiple words: `/poll \"whats your favorite color\" red blue green2`\n\n")

	// No SlashCommandID resolver: the docs are plain markdown and a command
	// mention would render as raw markup on the site.
	stdHelpFmt := &dcmd.StdHelpFormatter{}
	mockCmdData := &dcmd.Data{}

	for _, set := range sets {

		out.WriteString("## ")
		out.WriteString(set.Name())
		out.WriteString(" ")
		out.WriteString(set.Emoji())
		out.WriteString("\n\n")

		for _, entry := range set.Commands {
			// get the main name
			nameStr := entry.Container.FullName(false)
			if nameStr != "" {
				nameStr += " "
			}
			nameStr += entry.Cmd.Trigger.Names[0]
			// match how discord registers them
			nameStr = strings.ToLower(nameStr)

			// then aliases
			var as bytes.Buffer
			for _, alias := range entry.Cmd.Trigger.Names[1:] {
				as.WriteString("- " + alias + "\n")
			}
			aliases := as.String()

			// arguments and switches
			args := stdHelpFmt.ArgDefs(entry.Cmd, mockCmdData)
			switches := stdHelpFmt.Switches(entry.Cmd.Command)

			// ArgDefs only knows the command's own name, so commands inside a
			// container would otherwise be documented without the container.
			args = qualifyUsageLines(args, strings.ToLower(entry.Container.FullName(false)))

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

			anchor := strings.ReplaceAll(nameStr, " ", "-")
			out.WriteString("### " + nameStr + "\n\n")
			if aliases != "" {
				out.WriteString("#### Aliases{#" + anchor + "-aliases}\n\n" + aliases + "\n")
			}

			out.WriteString(desc)
			out.WriteString("\n")

			out.WriteString("#### Usage{#" + anchor + "-usage}\n\n")
			out.WriteString("```txt\n" + args + "\n```\n")

			if switches != "" {
				out.WriteString("\n```txt\n" + switches + "\n```\n")
			}
			out.WriteString("\n")
		}
	}

	os.Stdout.Write(out.Bytes())
}

// qualifyUsageLines prefixes every usage line with the container the command
// lives in, so "Roll <Sides>" reads "fun Roll <Sides>".
func qualifyUsageLines(usage, container string) string {
	if container == "" || usage == "" {
		return usage
	}

	lines := strings.Split(usage, "\n")
	for i, line := range lines {
		if line != "" && !strings.HasPrefix(line, container+" ") {
			lines[i] = container + " " + line
		}
	}
	return strings.Join(lines, "\n")
}

func GenConfigDocs() {

	keys := make([]string, 0, len(config.Singleton.Options))
	for k, _ := range config.Singleton.Options {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var out bytes.Buffer

	for _, k := range keys {
		v := config.Singleton.Options[k]

		out.WriteString("**" + v.Description + "**")

		typeStr := ""
		def := ""
		switch t := v.DefaultValue.(type) {
		case string:
			typeStr = "string"
			def = t
		case bool:
			typeStr = "true/false"
			def = "true"
			if !t {
				def = "false"
			}
		case int, uint, float32, float64, int8, int16, int32, int64, uint8, uint16, uint32, uint64:
			typeStr = "number"
			def = fmt.Sprint(t)
		}

		if typeStr != "" {
			out.WriteString(" (" + typeStr)
			if def != "" {
				out.WriteString(", default: " + def)
			}
			out.WriteString(")")
		}
		out.WriteString("\n")

		properKey := strings.ToUpper(v.Name)
		properKey = strings.ReplaceAll(properKey, ".", "_")
		out.WriteString(properKey + "\n\n")
	}

	os.Stdout.Write(out.Bytes())
}
