package dcmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// HelpFormatter is a interface for help formatters, for an example see StdHelpFormatter
type HelpFormatter interface {
	// Called when there is help generated for 2 or more commands
	ShortCmdHelp(cmd *RegisteredCommand, container *Container, data *Data) string

	// Called when help is only generated for 1 command
	// You are supposed to dump all command detilas such as arguments
	// the long description if it has one, switches and whatever else you have in mind.
	FullCmdHelp(cmd *RegisteredCommand, container *Container, data *Data) *discordgo.MessageEmbed
}

// SortedCommandEntry represents an entry in the SortdCommandSet
type SortedCommandEntry struct {
	Cmd       *RegisteredCommand
	Container *Container
}

// SortedCommandSet groups a set of commands by either container or category
type SortedCommandSet struct {
	Commands []*SortedCommandEntry

	// Set if this is a set of commands grouped by categories
	Category *Category

	// Set if this is a container helpContainer
	Container *Container
}

func (s *SortedCommandSet) Name() string {
	if s.Category != nil {
		return s.Category.Name
	}

	return s.Container.FullName(false)
}

func (s *SortedCommandSet) Color() int {
	if s.Category != nil {
		return s.Category.EmbedColor
	}

	return s.Container.HelpColor
}

func (s *SortedCommandSet) Emoji() string {
	if s.Category != nil {
		return s.Category.HelpEmoji
	}

	return s.Container.HelpTitleEmoji
}

// SortCommands groups commands into sorted command sets
func SortCommands(closestGroupContainer *Container, cmdContainer *Container) []*SortedCommandSet {
	containers := make([]*SortedCommandSet, 0)

	for _, cmd := range cmdContainer.Commands {
		if cmd.Trigger.HideFromHelp {
			continue
		}

		var keyCont *Container
		var keyCat *Category
		// Merge this containers generated command sets into the current one
		if c, ok := cmd.Command.(*Container); ok {
			topGroup := closestGroupContainer
			if c.HelpOwnEmbed {
				topGroup = c
			}
			merging := SortCommands(topGroup, c)
			for _, mergingSet := range merging {
				if set := FindSortedCommands(containers, mergingSet.Category, mergingSet.Container); set != nil {
					set.Commands = append(set.Commands, mergingSet.Commands...)
				} else {
					containers = append(containers, mergingSet)
				}
			}

			continue
		}

		// Check if this command belongs to a specific category
		if catCmd, ok := cmd.Command.(CmdWithCategory); ok {
			keyCat = catCmd.Category()
		}

		if keyCat == nil {
			keyCont = closestGroupContainer
		}

		if set := FindSortedCommands(containers, keyCat, keyCont); set != nil {
			set.Commands = append(set.Commands, &SortedCommandEntry{Cmd: cmd, Container: cmdContainer})
			continue
		}

		containers = append(containers, &SortedCommandSet{
			Commands:  []*SortedCommandEntry{{Cmd: cmd, Container: cmdContainer}},
			Category:  keyCat,
			Container: keyCont,
		})
	}

	return containers
}

// FindSortedCommands finds a command set by category or container
func FindSortedCommands(sets []*SortedCommandSet, cat *Category, container *Container) *SortedCommandSet {
	for _, set := range sets {
		if cat != nil && cat != set.Category {
			continue
		}

		if container != nil && container != set.Container {
			continue
		}

		return set
	}

	return nil
}

// GenerateFullHelp generates full help for a container
func GenerateHelp(d *Data, container *Container, formatter HelpFormatter) (embeds []*discordgo.MessageEmbed) {

	// Footers render as plain text, so a mention prefix would show as raw markup
	// rather than the bot's name. Only an actual prefix trigger is worth echoing,
	// and only while prefix triggers still work.
	invoked := "/"
	if d != nil && d.TriggerType == TriggerTypePrefix && d.TraditionalTriggerData != nil &&
		d.TraditionalTriggerData.PrefixUsed != "" &&
		(d.System == nil || !d.System.DisablePrefixTrigger) {
		invoked = d.TraditionalTriggerData.PrefixUsed
	}

	sets := SortCommands(container, container)

	for _, set := range sets {
		cName := set.Emoji() + set.Name()
		if cName != "" {
			cName += " "
		}

		embed := &discordgo.MessageEmbed{
			Title: cName + "Help",
			Color: set.Color(),
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Do `" + invoked + "help cmd/container` for more detailed information on a command/group of commands",
			},
		}

		for _, entry := range set.Commands {
			embed.Description += formatter.ShortCmdHelp(entry.Cmd, entry.Container, d)
		}

		embeds = append(embeds, embed)
	}

	return
}

// GenerateSingleHelp generates help for a single command
func GenerateTargettedHelp(target string, d *Data, container *Container, formatter HelpFormatter) (embeds []*discordgo.MessageEmbed) {

	cmd, cmdContainer := container.AbsFindCommand(target)
	if cmd == nil {
		if container != nil {
			return GenerateHelp(d, cmdContainer, formatter)
		}
		return nil
	}

	// Returning nothing lets the caller report it the same way as an unknown name.
	if cmd.Trigger.HideFromTargettedHelp {
		return nil
	}

	embed := formatter.FullCmdHelp(cmd, cmdContainer, d)

	return []*discordgo.MessageEmbed{embed}
}

type StdHelpFormatter struct {
	// SlashCommandID resolves a top level command or container name to its
	// registered application command id, letting help render clickable command
	// mentions. Returning 0 falls back to plain text, which is required for
	// anything not registered as a slash command: discord shows the raw markup
	// when the id does not resolve.
	//
	// Mentions only render where markdown does, so this never applies to embed
	// titles or footers.
	SlashCommandID func(topLevelName string) int64
}

// slashMention returns a clickable command mention for nameStr, or "" when the
// command has no registered id. Discord keys the mention off the top level
// command id and carries the rest of the path in the label, so "fun roll"
// resolves through "fun".
func (s *StdHelpFormatter) slashMention(nameStr string) string {
	if s.SlashCommandID == nil || nameStr == "" {
		return ""
	}

	topLevel := nameStr
	if i := strings.IndexByte(nameStr, ' '); i > 0 {
		topLevel = nameStr[:i]
	}

	id := s.SlashCommandID(topLevel)
	if id == 0 {
		return ""
	}

	return "</" + nameStr + ":" + strconv.FormatInt(id, 10) + ">"
}

// cmdMention renders nameStr as a mention where one is available and as plain
// text otherwise, since discord shows raw markup for an id it cannot resolve.
func (s *StdHelpFormatter) cmdMention(nameStr string) string {
	if mention := s.slashMention(nameStr); mention != "" {
		return mention
	}
	return "**`" + nameStr + "`**"
}

var _ HelpFormatter = (*StdHelpFormatter)(nil)

func (s *StdHelpFormatter) FullCmdHelp(cmd *RegisteredCommand, container *Container, data *Data) *discordgo.MessageEmbed {

	// Add the short description, if available
	desc := ""
	if cast, ok := cmd.Command.(CmdWithDescriptions); ok {
		short, long := cast.Descriptions(data)
		if long != "" {
			desc = long
		} else if short != "" {
			desc = short
		} else {
			desc = "No description for this command"
		}
	}

	args := s.ArgDefs(cmd, data)
	switches := s.Switches(cmd.Command)

	nameStr := s.CmdNameString(cmd, container, false)
	embed := &discordgo.MessageEmbed{
		Title: nameStr,
	}

	// Embed titles are plain text and code blocks suppress markdown, so the top
	// of the description is the only place in this embed a mention can render.
	if mention := s.slashMention(nameStr); mention != "" {
		embed.Description += mention + "\n"
	}

	if args != "" {
		embed.Description += "```\n" + args + "\n```"
	}
	if switches != "" {
		embed.Description += "```\n" + switches + "\n```"
	}

	embed.Description += "\n" + desc

	return embed
}

func (s *StdHelpFormatter) ShortCmdHelp(cmd *RegisteredCommand, container *Container, data *Data) string {

	// Add the current container stack to the name
	nameStr := s.CmdNameString(cmd, container, false)

	// Add the short description, if available
	desc := ""
	if cast, ok := cmd.Command.(CmdWithDescriptions); ok {
		short, long := cast.Descriptions(data)
		if short != "" {
			desc = ": " + short
		} else if long != "" {
			desc = ": " + long
		}
	}

	return fmt.Sprintf("%s%s\n\n", s.cmdMention(nameStr), desc)
}

// CmdWithCanonicalName is implemented by commands that are reachable from more
// than one place in the tree, so help can name the path they are really invoked
// by rather than whichever alias the user happened to type.
type CmdWithCanonicalName interface {
	CanonicalName() string
}

func (s *StdHelpFormatter) CmdNameString(cmd *RegisteredCommand, container *Container, containerAliases bool) string {
	// A command registered in a container and again at the root under a legacy
	// alias resolves to whichever was matched, and the root one has no container
	// in its chain. Ask the command where it actually lives.
	if cast, ok := cmd.Command.(CmdWithCanonicalName); ok {
		if canonical := cast.CanonicalName(); canonical != "" {
			return strings.ToLower(canonical)
		}
	}

	// Add the current container stack to the name
	nameStr := container.FullName(containerAliases)
	if nameStr != "" {
		nameStr += " "
	}

	// Slash commands have no aliases, so listing them here would advertise names
	// that only work through the prefix and mention triggers.
	nameStr += cmd.FormatNames(false, "/")

	// Discord registers every command and container name lowercased, so help
	// should show the name as it is actually typed.
	return strings.ToLower(nameStr)
}

func (s *StdHelpFormatter) Switches(cmd Cmd) (str string) {
	cast, ok := cmd.(CmdWithSwitches)
	if !ok {
		return ""
	}

	switches := cast.Switches()

	for _, sw := range switches {
		str += "[-" + strings.ToLower(sw.Name) + " " + s.ArgDef(sw) + "]\n"
	}
	// Trim the last newline
	if len(str) > 0 && strings.HasSuffix(str, "\n") {
		str = str[:len(str)-1]
	}

	return
}

func (s *StdHelpFormatter) ArgDefs(cmd *RegisteredCommand, data *Data) (str string) {
	cast, ok := cmd.Command.(CmdWithArgDefs)
	if !ok {
		return ""
	}

	defs, req, combos := cast.ArgDefs(data)

	name := strings.ToLower(cmd.FormatNames(false, "/"))
	if canonical, ok := cmd.Command.(CmdWithCanonicalName); ok {
		if c := canonical.CanonicalName(); c != "" {
			name = strings.ToLower(c)
		}
	}

	if len(combos) > 0 {
		for _, combo := range combos {
			comboDefs := make([]*ArgDef, len(combo))
			for i, v := range combo {
				comboDefs[i] = defs[v]
			}
			str += name + " " + s.ArgDefLine(comboDefs, len(comboDefs)) + "\n"
		}
	} else {
		str = name + " " + s.ArgDefLine(defs, req)
	}
	// Trim the last newline
	if len(str) > 0 && strings.HasSuffix(str, "\n") {
		str = str[:len(str)-1]
	}

	return
}

func (s *StdHelpFormatter) ArgDefLine(argDefs []*ArgDef, required int) (str string) {
	for i, arg := range argDefs {
		if i != 0 {
			str += " "
		}

		sepEnd := ">"
		if i >= required {
			// Optional
			sepEnd = "]"
			str += "["
		} else {
			str += "<"
		}

		str += s.ArgDef(arg)
		str += sepEnd
	}

	return
}

func (s *StdHelpFormatter) ArgDef(arg *ArgDef) (str string) {
	tName := "Switch"
	if arg.Type != nil {
		tName = arg.Type.HelpName()
	}

	str = fmt.Sprintf("%s:%s", strings.ToLower(arg.Name), tName)
	if arg.Help != "" {
		str += " - " + arg.Help
	}

	return
}

type StdHelpCommand struct {
	SendFullInDM      bool
	SendTargettedInDM bool

	Formatter HelpFormatter
}

var (
	_ Cmd                 = (*StdHelpCommand)(nil)
	_ CmdWithDescriptions = (*StdHelpCommand)(nil)
	_ CmdWithArgDefs      = (*StdHelpCommand)(nil)
)

func NewStdHelpCommand() *StdHelpCommand {
	return &StdHelpCommand{
		Formatter: &StdHelpFormatter{},
	}
}

func (h *StdHelpCommand) Descriptions(data *Data) (string, string) {
	return "Shows short help for all commands, or a longer help for a specific command", "Shows help for all or a specific command" +
		"\n\n**Examples:**\n`help` - Shows a short summary about all commands\n`help info` - Shows a longer help message for info, can contain examples of how to use it.\nYou are currently reading the longer help message about the `help` command"
}

func (h *StdHelpCommand) ArgDefs(data *Data) (args []*ArgDef, required int, combos [][]int) {
	return []*ArgDef{
		{Name: "Command", Type: String},
	}, 0, nil
}

func (h *StdHelpCommand) Run(d *Data) (interface{}, error) {
	root := d.ContainerChain[0]

	target := d.Args[0].Str()

	var help []*discordgo.MessageEmbed
	if target != "" {
		help = GenerateTargettedHelp(target, d, root, h.Formatter)
	} else {
		help = GenerateHelp(d, root, h.Formatter)
	}

	return help, nil
}
