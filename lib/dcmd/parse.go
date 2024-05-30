package dcmd

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var (
	ErrNoComboFound       = NewSimpleUserError("No matching combo found")
	ErrNotEnoughArguments = NewSimpleUserError("Not enough arguments passed")
)

func ArgParserMW(inner RunFunc) RunFunc {
	return func(data *Data) (interface{}, error) {
		// Parse Args
		err := ParseCmdArgs(data)
		if err != nil {
			if IsUserError(err) {
				return "Invalid arguments provided: " + err.Error(), nil
			}

			return nil, err
		}

		return inner(data)

	}
}

func ParseCmdArgs(data *Data) error {
	switch data.TriggerType {
	case TriggerTypeSlashCommands:
		return ParseCmdArgsFromInteraction(data)
	default:
		return ParseCmdArgsFromMessage(data)
	}
}

func ParseCmdArgsFromInteraction(data *Data) error {
	sorted := SortInteractionOptions(data)

	// 	ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error)

	// Helper map to ease parsing of args
	optionsMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, def := range sorted {
		for _, opt := range def.interactionOptions {
			optionsMap[opt.Name] = opt
		}
	}

	opts := &SlashCommandsParseOptions{
		Options:     optionsMap,
		Interaction: data.SlashCommandTriggerData.Interaction,
	}

	data.Switches = make(map[string]*ParsedArg)

	// start the actual parsing
	for _, def := range sorted {
		parsedDef := def.def.NewParsedDef()

		if len(def.interactionOptions) < 1 {
			// no options provided by the user
			if def.required {
				return ErrNotEnoughArguments
			}
			// not a reuired arg, just skip parsing it
		} else {

			if def.def.Type == nil {
				parsed, err := opts.ExpectBool(def.def.Name)
				if err != nil {
					return err
				}
				parsedDef.Value = parsed
			} else {
				parsed, err := def.def.Type.ParseFromInteraction(def.def, data, opts)
				if err != nil {
					return err
				}

				parsedDef.Value = parsed
			}

		}

		if def.isSwitch {
			data.Switches[def.def.Name] = parsedDef
		} else {
			data.Args = append(data.Args, parsedDef)
		}
	}

	return nil
}

type sortedInteractionArg struct {
	key                    string
	interactionOptionNames []string
	interactionOptions     []*discordgo.ApplicationCommandInteractionDataOption
	def                    *ArgDef
	isSwitch               bool
	required               bool
}

func SortInteractionOptions(data *Data) []*sortedInteractionArg {
	sorted := make([]*sortedInteractionArg, 0)

	argDefsCommand, argDefsOk := data.Cmd.Command.(CmdWithArgDefs)

	if argDefsOk {
		defs, required, combos := argDefsCommand.ArgDefs(data)
		sortedDefs := sortInteractionArgDefs(data, defs, required, combos)
		sorted = append(sorted, sortedDefs...)
		// for k, v := range sortedDefs {
		// 	sorted[k] = v
		// }
	}

	switchesCmd, switchesOk := data.Cmd.Command.(CmdWithSwitches)
	if switchesOk {
		defs := switchesCmd.Switches()
		sortedDefs := sortInteractionArgDefs(data, defs, 0, nil)
		sorted = append(sorted, sortedDefs...)

		for _, v := range sortedDefs {
			v.isSwitch = true
		}
	}

	return sorted
}

func sortInteractionArgDefs(data *Data, defs []*ArgDef, required int, combos [][]int) []*sortedInteractionArg {
	sorted := make([]*sortedInteractionArg, 0)

	// For now we use the smallest argument combo for the required args
	// The proper way to handle this would be to register multiple commands like subcommands
	smallestCombo := findSmallestCombo(defs, required, combos)

	for i, v := range defs {
		isRequired := false
		if containsInt(smallestCombo, i) {
			isRequired = true
		}

		var defOpts []*discordgo.ApplicationCommandOption
		if v.Type != nil {
			defOpts = v.Type.SlashCommandOptions(v)
		} else {
			defOpts = []*discordgo.ApplicationCommandOption{v.StandardSlashCommandOption(discordgo.ApplicationCommandOptionBoolean)}
		}

		sortedEntry := &sortedInteractionArg{
			key:      v.Name,
			def:      v,
			required: isRequired,
		}

		// match the spec options to the actual provided interaction options
	OUTER:
		for _, do := range defOpts {
			sortedEntry.interactionOptionNames = append(sortedEntry.interactionOptionNames, do.Name)
			for _, iv := range data.SlashCommandTriggerData.Options {
				// Found a provided option that matched the arg def option
				if strings.EqualFold(iv.Name, do.Name) {
					sortedEntry.interactionOptions = append(sortedEntry.interactionOptions, iv)
					continue OUTER
				}
			}
		}

		sorted = append(sorted, sortedEntry)
	}

	return sorted
}

func containsInt(s []int, i int) bool {
	for _, v := range s {
		if v == i {
			return true
		}
	}

	return false
}

func findSmallestCombo(defs []*ArgDef, requiredArgs int, combos [][]int) []int {
	var smallest []int
	if len(combos) > 0 {
		first := true
		// combos takes presedence as to not break backwards compatibility
		for _, v := range combos {
			if first || len(v) < len(smallest) {
				smallest = v
				first = false
			}
		}

		return smallest
	}

	for i := 0; i < requiredArgs; i++ {
		smallest = append(smallest, i)
	}

	return smallest
}

// ParseCmdArgsFromMessage parses arguments from a MESSAGE
// will panic if used on slash commands
func ParseCmdArgsFromMessage(data *Data) error {
	if data.TraditionalTriggerData == nil {
		panic("ParseCmdArgsFromMessage used on context without TraditionalTriggerData")
	}

	argDefsCommand, argDefsOk := data.Cmd.Command.(CmdWithArgDefs)
	switchesCmd, switchesOk := data.Cmd.Command.(CmdWithSwitches)

	if !argDefsOk && !switchesOk {
		// Command dosen't use the standard arg parsing
		return nil
	}

	// Split up the args
	split := SplitArgs(data.TraditionalTriggerData.MessageStrippedPrefix)

	var err error
	if switchesOk {
		switches := switchesCmd.Switches()
		if len(switches) > 0 {
			// Parse the switches first
			split, err = ParseSwitches(switchesCmd.Switches(), data, split)
			if err != nil {
				return err
			}
		}
	}

	if argDefsOk {
		defs, req, combos := argDefsCommand.ArgDefs(data)
		if len(defs) > 0 {
			err = ParseArgDefs(defs, req, combos, data, split)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ParseArgDefs parses ordered argument definition for a CmdWithArgDefs
func ParseArgDefs(defs []*ArgDef, required int, combos [][]int, data *Data, split []*RawArg) error {

	combo, ok := FindCombo(defs, combos, split)
	if !ok {
		return ErrNoComboFound
	}

	parsedArgs := NewParsedArgs(defs)
	for i, v := range combo {
		def := defs[v]
		if i >= len(split) {
			if i >= required && len(combos) < 1 {
				break
			}
			return ErrNotEnoughArguments
		}

		combined := ""
		if i == len(combo)-1 && len(split)-1 > i {
			// Last arg, but still more after, combine and rebuilt them
			for j := i; j < len(split); j++ {
				if j != i {
					combined += " "
				}

				temp := split[j]
				if temp.Container != 0 {
					combined += string(temp.Container) + temp.Str + string(temp.Container)
				} else {
					combined += temp.Str
				}
			}
		} else {
			combined = split[i].Str
		}

		val, err := def.Type.ParseFromMessage(def, combined, data)
		if err != nil {
			return err
		}
		parsedArgs[v].Value = val
	}

	data.Args = parsedArgs

	return nil
}

// ParseSwitches parses all switches for a CmdWithSwitches, and also takes them out of the raw args
func ParseSwitches(switches []*ArgDef, data *Data, split []*RawArg) ([]*RawArg, error) {
	newRaws := make([]*RawArg, 0, len(split))

	// Initialise the parsed switches
	parsedSwitches := make(map[string]*ParsedArg)
	for _, v := range switches {
		parsedSwitches[v.Name] = &ParsedArg{
			Value: v.Default,
			Def:   v,
		}
	}

	for i := 0; i < len(split); i++ {
		raw := split[i]
		if raw.Container != 0 {
			newRaws = append(newRaws, raw)
			continue
		}

		if !strings.HasPrefix(raw.Str, "-") {
			newRaws = append(newRaws, raw)
			continue
		}

		rest := raw.Str[1:]
		var matchedArg *ArgDef
		for _, v := range switches {
			if v.Name == rest {
				matchedArg = v
				break
			}
		}

		if matchedArg == nil {
			newRaws = append(newRaws, raw)
			continue
		}

		if matchedArg.Type == nil {
			parsedSwitches[matchedArg.Name].Raw = raw
			parsedSwitches[matchedArg.Name].Value = true
			continue
		}

		if i >= len(split)-1 {
			// A switch with extra stuff requird, but no extra data provided
			// Can't handle this case...
			continue
		}

		// At this point, we have encountered a switch with data
		// so we need to skip the next RawArg
		i++

		val, err := matchedArg.Type.ParseFromMessage(matchedArg, split[i].Str, data)
		if err != nil {
			// TODO: Use custom error type for helpfull errror
			return nil, err
		}

		parsedSwitches[matchedArg.Name].Raw = raw
		parsedSwitches[matchedArg.Name].Value = val
	}
	data.Switches = parsedSwitches
	return newRaws, nil
}

func isArgContainer(r rune) bool {
	return r == '"' || r == '`'
}

type RawArg struct {
	Str       string
	Container rune
}

// SplitArgs splits the string into fields
func SplitArgs(in string) []*RawArg {
	var rawArgs []*RawArg

	var buf strings.Builder
	escape := false
	var container rune
	for _, r := range in {
		// Apply or remove escape mode
		if r == '\\' {
			if escape {
				escape = false
				buf.WriteByte('\\')
			} else {
				escape = true
			}

			continue
		}

		// Check for other special tokens
		isSpecialToken := true
		if r == ' ' {
			// Maybe separate by space
			if buf.Len() > 0 && container == 0 && !escape {
				rawArgs = append(rawArgs, &RawArg{buf.String(), 0})
				buf.Reset()
			} else if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
		} else if r == container && container != 0 {
			// Split arg here
			if escape {
				buf.WriteRune(r)
			} else {
				rawArgs = append(rawArgs, &RawArg{buf.String(), container})
				buf.Reset()
				container = 0
			}
		} else if container == 0 && buf.Len() == 0 {
			// Check if we should start containing a arg
			if isArgContainer(r) {
				if escape {
					buf.WriteRune(r)
				} else {
					container = r
				}
			} else {
				isSpecialToken = false
			}
		} else {
			isSpecialToken = false
		}

		if !isSpecialToken {
			if escape {
				buf.WriteByte('\\')
			}
			buf.WriteRune(r)
		}

		// Reset escape mode
		escape = false
	}

	// Something was left in the buffer just add it to the end
	if buf.Len() > 0 {
		item := buf.String()
		if container != 0 {
			item = string(container) + item
		}
		rawArgs = append(rawArgs, &RawArg{item, 0})
	}

	return rawArgs
}

type comboStats struct {
	combo  []int
	compat struct{ poorMatches, goodMatches int }
}

func (c1 *comboStats) BetterThan(c2 *comboStats) bool {
	compat1, compat2 := c1.compat, c2.compat
	if compat1.goodMatches != compat2.goodMatches {
		return compat1.goodMatches > compat2.goodMatches
	}
	return compat1.poorMatches > compat2.poorMatches
}

// Finds a proper argument combo from the provided args
func FindCombo(defs []*ArgDef, combos [][]int, args []*RawArg) (combo []int, ok bool) {
	if len(combos) < 1 {
		out := make([]int, len(defs))
		for k := range out {
			out[k] = k
		}
		return out, true
	}

	bestStats := new(comboStats)
	for _, combo := range combos {
		stats, comboOK := collectComboStats(combo, defs, args)
		if !comboOK {
			continue
		}
		if !ok || stats.BetterThan(bestStats) {
			bestStats = stats
			ok = true
		}
	}

	return bestStats.combo, ok
}

func collectComboStats(combo []int, defs []*ArgDef, args []*RawArg) (stats *comboStats, ok bool) {
	if len(combo) > len(args) {
		return nil, false
	}

	stats = &comboStats{combo: combo}
	for i, defPos := range combo {
		def := defs[defPos]
		switch compat := def.Type.CheckCompatibility(def, args[i].Str); compat {
		case Incompatible:
			return nil, false
		case CompatibilityPoor:
			stats.compat.poorMatches++
		case CompatibilityGood:
			stats.compat.goodMatches++
		default:
			panic(fmt.Sprintf("dcmd: got unexpected compatibility result while selecting combo: %s", compat))
		}
	}
	return stats, true
}
