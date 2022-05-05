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
		properKey = strings.Replace(properKey, ".", "_", -1)
		out.WriteString(properKey + "\n\n")
	}

	os.Stdout.Write(out.Bytes())
}
