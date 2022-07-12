package dcmd

import (
	"reflect"
	"strings"
)

// RegisteredCommand represents a registered command to the system.
// RegisteredCommand.Cmd may exist in other RegisteredCommands but the RegisteredCommand wrapper itself
// is unique per route
type RegisteredCommand struct {
	Command                  Cmd
	Trigger                  *Trigger
	builtFullMiddlewareChain RunFunc
}

// FormatNames returns a string with names and if includedAliases is true, aliases seperated by seperator
// Falls back to reflection if no names are available
func (r *RegisteredCommand) FormatNames(includeAliases bool, seperator string) string {
	if len(r.Trigger.Names) > 0 {
		if includeAliases {
			return strings.Join(r.Trigger.Names, seperator)
		}

		return r.Trigger.Names[0]
	}

	t := reflect.TypeOf(r.Command)
	return t.Name()
}

// Cmd is the interface all commands must implement to be considered a command
type Cmd interface {
	// Run the command with the provided data,
	// response is either string, error, embed or custom that implements CmdResponse
	// if response is nil, and an error is returned that is not PublicError,
	// the string "An error occured. Contact bot owner" becomes the response
	Run(data *Data) (interface{}, error)
}

// CmdWithDescriptions commands will have the descriptions used in the standard help generator
// short is used for non-targetted help while long is used for targetted help
type CmdWithDescriptions interface {
	Descriptions(data *Data) (short, long string)
}

// CmdWithArgDefs commands will have their arguments parsed  following the argdefs, required and combos rules returned
// if parsing fails, or the conditions are not met, the command will not be invoked, and instead return the reason it failed, and the short help for the command
type CmdWithArgDefs interface {

	/*
		Returns the argument definitions

		if 'combos' is non nil, then that takes priority over 'required'
		Combos wllows the command to take different combinations and orders of arguments

		Example: a `clean` command that deletes x amount of messages and can optionally filter by user
		could look like this
		```
		argDefs = []*ArgDef{
		    {Name: "limit", Type: Int, Default: 10},
		    {Name: "user", Type: User, RequireMention: true},
		}
		requiredArgs = 0
		```
		Here the clean command can take any number of arguments, if no arguments are passed, limit will be 10, and user will be nil.
		if only a limit is provided, then user will be nil and
		Here we have the 2 arguments, and atm we can only invoke this as `clean x` or `clean x @user`
		using combos we can also allow any order here, this is possible because the user arg cannot be a number.
		so if we use the combos:
		```
		combos = [][]int{
			[]int{0},   // Allow only a limit to be passed
			[]int{0,1}, // Allow both a limit and user to be passed, in that order
			[]int{1},   // Allow only a user to be passed, then limit will have its default value of 10
			[]int{1,0}, // Allow both a user and limit to be passed, in that order
			[]int{},    // Allow no arguments to be passe, limit will then have its default of 10, and user will be nil
		}
		```
		As you can see, the integers are indexes in the argument definitions returned. The above combos will allow any combination or arguments, even none.

		Argument combos generally work as expected in non-ambiguous cases, but it cannot for example detect the difference between two text arguments.
	*/
	ArgDefs(data *Data) (args []*ArgDef, required int, combos [][]int)
}

/*
CmdWithSwitches commands can define a set of switches, which might be a cleaner alternative to combos
This will also set the context value of ContextStrippedSwitches to the arg string without the switches, for further parsing of the remaining switch-less args

if you don't know what a switch is, clean command example using both a switch and single argdef could look like the following:
`clean 10`          - clean 10 last messages without a user specified
`clean -u @user 10` - specify a user, but still clean 10 messages

This is a lot cleaner than using argument combos
*/
type CmdWithSwitches interface {
	Switches() []*ArgDef
}

// CmdWithCanUse commands can use this to hide certain commands from people for example
type CmdWithCanUse interface {
	CanUse(data *Data) (bool, error)
}

// CmdWithCustomParser is for commands that want to implement their own custom argument parser
type CmdWithCustomParser interface {

	// Parse the arguments and return the command data
	// sripped is the rest of the message with the command name removed
	Parse(stripped string, data *Data) (*Data, error)
}

// CmdWithCategory puts the command in a category, mostly used for the help generation
type CmdWithCategory interface {
	Category() *Category
}

// Category represents a command category
type Category struct {
	Name        string
	Description string
	HelpEmoji   string
	EmbedColor  int
}

// CmdLongArgDefs is a helper for easily adding a compile time assertion
// Example: `var _ CmdLongArgDefs = (YourCommandType)(nil)`
// will fail to compile if you do not implement this interface correctly
type CmdDescArgDefs interface {
	Cmd

	CmdWithDescriptions
	CmdWithArgDefs
}
