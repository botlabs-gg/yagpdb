package dcmd

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

// ArgDef represents a argument definition, either a switch or plain arg
type ArgDef struct {
	Name    string
	Type    ArgType
	Help    string
	Default interface{}
}

func (def *ArgDef) StandardSlashCommandOption(typ discordgo.ApplicationCommandOptionType) *discordgo.ApplicationCommandOption {
	desc := cutStringShort(def.Help, 100)
	if desc == "" {
		desc = def.Name
	}

	return &discordgo.ApplicationCommandOption{
		Name:        def.Name,
		Description: desc,
		Type:        typ,
	}
}

// CutStringShort stops a strinng at "l"-3 if it's longer than "l" and adds "..."
func cutStringShort(s string, l int) string {
	var mainBuf bytes.Buffer
	var latestBuf bytes.Buffer

	i := 0
	for _, r := range s {
		latestBuf.WriteRune(r)
		if i > 3 {
			lRune, _, _ := latestBuf.ReadRune()
			mainBuf.WriteRune(lRune)
		}

		if i >= l {
			return mainBuf.String() + "..."
		}
		i++
	}

	return mainBuf.String() + latestBuf.String()
}

func (def *ArgDef) NewParsedDef() *ParsedArg {
	return &ParsedArg{
		Def:   def,
		Value: def.Default,
	}
}

type ParsedArg struct {
	Def   *ArgDef
	Value interface{}
	Raw   *RawArg
}

func (p *ParsedArg) Str() string {
	if p.Value == nil {
		return ""
	}

	switch t := p.Value.(type) {
	case string:
		return t
	case int, int32, int64, uint, uint32, uint64:
		return strconv.FormatInt(p.Int64(), 10)
	default:
		return ""
	}
}

// TODO: GO-Generate the number ones
func (p *ParsedArg) Int() int {
	if p.Value == nil {
		return 0
	}

	switch t := p.Value.(type) {
	case int:
		return t
	case uint:
		return int(t)
	case int32:
		return int(t)
	case int64:
		return int(t)
	case uint32:
		return int(t)
	case uint64:
		return int(t)
	default:
		return 0
	}
}

func (p *ParsedArg) Int64() int64 {
	if p.Value == nil {
		return 0
	}

	switch t := p.Value.(type) {
	case int:
		return int64(t)
	case uint:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case uint32:
		return int64(t)
	case uint64:
		return int64(t)
	default:
		return 0
	}
}

func (p *ParsedArg) Bool() bool {
	if p.Value == nil {
		return false
	}

	switch t := p.Value.(type) {
	case bool:
		return t
	case int, int32, int64, uint, uint32, uint64:
		return p.Int64() > 0
	case string:
		return t != ""
	}

	return false
}

func (p *ParsedArg) MemberState() *dstate.MemberState {
	if p.Value == nil {
		return nil
	}

	switch t := p.Value.(type) {
	case *dstate.MemberState:
		return t
	case *AdvUserMatch:
		return t.Member
	}

	return nil
}

func (p *ParsedArg) User() *discordgo.User {
	if p.Value == nil {
		return nil
	}

	switch t := p.Value.(type) {
	case *dstate.MemberState:
		return &t.User
	case *AdvUserMatch:
		return t.User
	}

	return nil
}

func (p *ParsedArg) AdvUser() *AdvUserMatch {
	if p.Value == nil {
		return nil
	}

	switch t := p.Value.(type) {
	case *AdvUserMatch:
		return t
	}

	return nil
}

// NewParsedArgs creates a new ParsedArg slice from defs passed, also filling default values
func NewParsedArgs(defs []*ArgDef) []*ParsedArg {
	out := make([]*ParsedArg, len(defs))

	for k := range out {
		out[k] = defs[k].NewParsedDef()
	}

	return out
}

type SlashCommandsParseOptions struct {
	Options     map[string]*discordgo.ApplicationCommandInteractionDataOption
	Interaction *discordgo.Interaction
}

func (sopts *SlashCommandsParseOptions) ExpectAny(name string) (interface{}, error) {
	if v, ok := sopts.ExpectAnyOpt(name); ok {
		return v, nil
	} else {
		return 0, NewErrArgExpected(name, "any", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectAnyOpt(name string) (interface{}, bool) {
	if v, ok := sopts.Options[strings.ToLower(name)]; ok {
		return v.Value, true
	}

	return nil, false

}

func (sopts *SlashCommandsParseOptions) ExpectInt64(name string) (int64, error) {
	if v, found, err := sopts.ExpectInt64Opt(name); err != nil {
		return 0, err
	} else if found {
		return v, nil
	} else {
		return 0, NewErrArgExpected(name, "int64", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectInt64Opt(name string) (int64, bool, error) {
	if v, ok := sopts.Options[strings.ToLower(name)]; ok {
		if cast, ok2 := v.Value.(int64); ok2 {
			return cast, true, nil
		} else {
			return 0, true, NewErrArgExpected(name, "int64", v.Value)
		}
	}

	return 0, false, nil
}

func (sopts *SlashCommandsParseOptions) ExpectString(name string) (string, error) {
	if v, found, err := sopts.ExpectStringOpt(name); err != nil {
		return "", err
	} else if found {
		return v, nil
	} else {
		return "", NewErrArgExpected(name, "string", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectStringOpt(name string) (string, bool, error) {
	if v, ok := sopts.Options[strings.ToLower(name)]; ok {
		if cast, ok2 := v.Value.(string); ok2 {
			return cast, true, nil
		} else {
			return "", true, NewErrArgExpected(name, "string", v.Value)
		}
	}

	return "", false, nil
}

func (sopts *SlashCommandsParseOptions) ExpectBool(name string) (bool, error) {
	if v, found, err := sopts.ExpectBoolOpt(name); err != nil {
		return false, err
	} else if found {
		return v, nil
	} else {
		return false, NewErrArgExpected(name, "bool", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectBoolOpt(name string) (bool, bool, error) {
	if v, ok := sopts.Options[strings.ToLower(name)]; ok {
		if cast, ok2 := v.Value.(bool); ok2 {
			return cast, true, nil
		} else {
			return false, true, NewErrArgExpected(name, "string", v.Value)
		}
	}

	return false, false, nil
}

func (sopts *SlashCommandsParseOptions) ExpectUser(name string) (*discordgo.User, error) {
	if v, found, err := sopts.ExpectUserOpt(name); err != nil {
		return nil, err
	} else if found {
		return v, nil
	} else {
		return nil, NewErrArgExpected(name, "*discordgo.User", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectUserOpt(name string) (*discordgo.User, bool, error) {
	id, found, err := sopts.ExpectInt64Opt(name)
	if err != nil || !found {
		return nil, found, err
	}

	user, ok := sopts.Interaction.DataCommand.Resolved.Users[id]
	if !ok {
		return nil, true, &ErrResolvedNotFound{Key: name, ID: id, Type: "user"}
	}

	return user, true, nil
}

func (sopts *SlashCommandsParseOptions) ExpectMember(name string) (*discordgo.Member, error) {

	if v, found, err := sopts.ExpectMemberOpt(name); err != nil {
		return nil, err
	} else if found {
		return v, nil
	} else {
		return nil, NewErrArgExpected(name, "*discordgo.Member", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectMemberOpt(name string) (*discordgo.Member, bool, error) {
	id, found, err := sopts.ExpectInt64Opt(name)
	if err != nil || !found {
		return nil, found, err
	}

	member, ok := sopts.Interaction.DataCommand.Resolved.Members[id]
	if !ok {
		return nil, true, &ErrResolvedNotFound{Key: name, ID: id, Type: "member"}
	}

	user, ok := sopts.Interaction.DataCommand.Resolved.Users[id]
	if !ok && member.User == nil {
		return nil, true, &ErrResolvedNotFound{Key: name, ID: id, Type: "user"}
	}

	member.User = user

	return member, true, nil
}

func (sopts *SlashCommandsParseOptions) ExpectRole(name string) (*discordgo.Role, error) {
	if v, found, err := sopts.ExpectRoleOpt(name); err != nil {
		return nil, err
	} else if found {
		return v, nil
	} else {
		return nil, NewErrArgExpected(name, "*discordgo.Role", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectRoleOpt(name string) (*discordgo.Role, bool, error) {
	id, found, err := sopts.ExpectInt64Opt(name)
	if err != nil || !found {
		return nil, found, err
	}

	user, ok := sopts.Interaction.DataCommand.Resolved.Roles[id]
	if !ok {
		return nil, true, &ErrResolvedNotFound{Key: name, ID: id, Type: "role"}
	}

	return user, true, nil
}

func (sopts *SlashCommandsParseOptions) ExpectChannel(name string) (*discordgo.Channel, error) {
	if v, found, err := sopts.ExpectChannelOpt(name); err != nil {
		return nil, err
	} else if found {
		return v, nil
	} else {
		return nil, NewErrArgExpected(name, "*discordgo.Channel", nil)
	}
}

func (sopts *SlashCommandsParseOptions) ExpectChannelOpt(name string) (*discordgo.Channel, bool, error) {
	id, found, err := sopts.ExpectInt64Opt(name)
	if err != nil || !found {
		return nil, found, err
	}

	user, ok := sopts.Interaction.DataCommand.Resolved.Channels[id]
	if !ok {
		return nil, true, &ErrResolvedNotFound{Key: name, ID: id, Type: "channel"}
	}

	return user, true, nil
}

// ArgType is the interface argument types has to implement,
type ArgType interface {
	// CheckCompatibility reports the degree to which the input matches the type.
	CheckCompatibility(def *ArgDef, part string) CompatibilityResult

	// Attempt to parse it, returning any error if one occured.
	ParseFromMessage(def *ArgDef, part string, data *Data) (val interface{}, err error)
	ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error)

	// Name as shown in help
	HelpName() string

	SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption
}

// CompatibilityResult indicates the degree to which a value matches a type.
type CompatibilityResult int

const (
	// Incompatible indicates that the value does not match the type at all. For example,
	// the string "abc" would be incompatible with an integer type.
	Incompatible CompatibilityResult = iota

	// CompatibilityPoor indicates that the value superficially matches the
	// type, but violates some required constraint or property. For example, 11
	// would have poor compatibility with an integer type with range limited to
	// [0, 10].
	CompatibilityPoor

	// CompatibilityGood indicates that the value matches the type well. For
	// example, 204255221017214977 would have good compatibility with a user
	// type as it has the correct length for a Discord snowflake though the ID
	// may not correspond to a valid user. Similarly, 10 would have good compatibility
	// with an unbounded integer type.
	CompatibilityGood
)

func (c CompatibilityResult) String() string {
	switch c {
	case Incompatible:
		return "incompatible"
	case CompatibilityPoor:
		return "poor compatibility"
	case CompatibilityGood:
		return "good compatibility"
	default:
		return fmt.Sprintf("CompatibilityResult(%d)", c)
	}
}

// DetermineSnowflakeCompatibility returns CompatibilityGood if s could represent
// a Discord snowflake ID and Incompatible otherwise.
func DetermineSnowflakeCompatibility(s string) CompatibilityResult {
	_, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Incompatible
	}

	const (
		minSnowflakeLength = 17
		maxSnowflakeLength = 19
	)
	if len(s) < minSnowflakeLength || len(s) > maxSnowflakeLength {
		return CompatibilityPoor
	}
	return CompatibilityGood
}

var (
	// Create some convenience instances
	Int             = &IntArg{}
	BigInt          = &IntArg{InteractionString: true}
	Float           = &FloatArg{}
	String          = &StringArg{}
	User            = &UserArg{}
	UserID          = &UserIDArg{}
	Channel         = &ChannelArg{}
	ChannelOrThread = &ChannelArg{AllowThreads: true}
	AdvUser         = &AdvUserArg{EnableUserID: true, EnableUsernameSearch: true, RequireMembership: true}
	AdvUserNoMember = &AdvUserArg{EnableUserID: true, EnableUsernameSearch: true}
)

// IntArg matches and parses integer arguments
// If min and max are not equal then the value has to be within min and max or else it will fail parsing
type IntArg struct {
	Min, Max int64

	// if we wanna support large numbers like snowflakes we have to use strings with interactions
	InteractionString bool
}

var _ ArgType = (*IntArg)(nil)

func (i *IntArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	v, err := strconv.ParseInt(part, 10, 64)
	if err != nil {
		return Incompatible
	}
	if i.Min == i.Max || i.Min <= v && v <= i.Max {
		return CompatibilityGood
	}
	return CompatibilityPoor
}

func (i *IntArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {
	v, err := strconv.ParseInt(part, 10, 64)
	if err != nil {
		return nil, &InvalidInt{part}
	}

	// A valid range has been specified
	if i.Max != i.Min {
		if i.Max < v || i.Min > v {
			return nil, &OutOfRangeError{ArgName: def.Name, Got: v, Min: i.Min, Max: i.Max}
		}
	}

	return v, nil
}

func (i *IntArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {

	any, err := options.ExpectAny(def.Name)
	if err != nil {
		return nil, err
	}

	var v int64
	switch t := any.(type) {
	case string:
		v, err = strconv.ParseInt(t, 10, 64)
		if err != nil {
			return nil, err
		}
	case int64:
		v = t
	default:
	}

	// A valid range has been specified
	if i.Max != i.Min {
		if i.Max < v || i.Min > v {
			return nil, &OutOfRangeError{ArgName: def.Name, Got: v, Min: i.Min, Max: i.Max}
		}
	}

	return v, nil
}

func (i *IntArg) HelpName() string {
	return "Whole number"
}

func (i *IntArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	if i.InteractionString {
		return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionString)}
	}
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionInteger)}
}

// FloatArg matches and parses float arguments
// If min and max are not equal then the value has to be within min and max or else it will fail parsing
type FloatArg struct {
	Min, Max float64
}

var _ ArgType = (*FloatArg)(nil)

func (f *FloatArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	v, err := strconv.ParseFloat(part, 64)
	if err != nil {
		return Incompatible
	}
	if f.Min == f.Max || f.Min <= v && v <= f.Max {
		return CompatibilityGood
	}
	return CompatibilityPoor
}

func (f *FloatArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {
	v, err := strconv.ParseFloat(part, 64)
	if err != nil {
		return nil, &InvalidFloat{part}
	}

	// A valid range has been specified
	if f.Max != f.Min {
		if f.Max < v || f.Min > v {
			return nil, &OutOfRangeError{ArgName: def.Name, Got: v, Min: f.Min, Max: f.Max, Float: true}
		}
	}

	return v, nil
}

func (f *FloatArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {
	v, err := options.ExpectString(def.Name)
	if err != nil {
		return nil, err
	}

	parsedF, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, err
	}

	// A valid range has been specified
	if f.Max != f.Min {
		if f.Max < parsedF || f.Min > parsedF {
			return nil, &OutOfRangeError{ArgName: def.Name, Got: parsedF, Min: f.Min, Max: f.Max, Float: true}
		}
	}

	return parsedF, nil
}

func (f *FloatArg) HelpName() string {
	return "Decimal number"
}

func (f *FloatArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionString)}
}

// StringArg matches and parses float arguments
type StringArg struct{}

var _ ArgType = (*StringArg)(nil)

func (s *StringArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	return CompatibilityGood
}

func (s *StringArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {
	return part, nil
}

func (s *StringArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {
	v, err := options.ExpectString(def.Name)
	return v, err
}

func (s *StringArg) HelpName() string {
	return "Text"
}

func (s *StringArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionString)}
}

// UserArg matches and parses user argument (mention/ID)
type UserArg struct{}

var _ ArgType = (*UserArg)(nil)

func (u *UserArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	if strings.HasPrefix(part, "<@") && strings.HasSuffix(part, ">") {
		return DetermineSnowflakeCompatibility(strings.TrimPrefix(part, "!"))
	}
	return DetermineSnowflakeCompatibility(part)
}

func (u *UserArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {
	if strings.HasPrefix(part, "<@") && len(part) > 3 {
		// Direct mention
		id := part[2 : len(part)-1]
		if id[0] == '!' {
			// Nickname mention
			id = id[1:]
		}

		parsed, _ := strconv.ParseInt(id, 10, 64)
		for _, v := range data.TraditionalTriggerData.Message.Mentions {
			if parsed == v.ID {
				return v, nil
			}
		}
		return nil, &ImproperMention{part}
	}

	id, err := strconv.ParseInt(part, 10, 64)
	if err != nil {
		return nil, err
	}

	member := data.System.State.GetMember(data.GuildData.GS.ID, id)
	if member != nil {
		return &member.User, nil
	}

	m, err := data.Session.GuildMember(data.GuildData.GS.ID, id)
	if err == nil {
		member = dstate.MemberStateFromMember(m)
		return &member.User, nil
	}
	return nil, err
}

func (u *UserArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {
	user, err := options.ExpectUser(def.Name)
	return user, err
}

func (u *UserArg) HelpName() string {
	return "User"
}

func (u *UserArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionUser)}
}

var CustomUsernameSearchFunc func(state dstate.StateTracker, gs *dstate.GuildSet, query string) (*dstate.MemberState, error)

func FindDiscordMemberByName(state dstate.StateTracker, gs *dstate.GuildSet, str string) (*dstate.MemberState, error) {
	if CustomUsernameSearchFunc != nil {
		return CustomUsernameSearchFunc(state, gs, str)
	}

	lowerIn := strings.ToLower(str)

	partialMatches := make([]*dstate.MemberState, 0, 5)
	fullMatches := make([]*dstate.MemberState, 0, 5)

	state.IterateMembers(gs.ID, func(chunk []*dstate.MemberState) bool {
		for _, v := range chunk {
			if v == nil || v.Member == nil {
				continue
			}

			if v.User.Username == "" {
				continue
			}

			if strings.EqualFold(str, v.User.Username) || strings.EqualFold(str, v.Member.Nick) {
				fullMatches = append(fullMatches, v)
				if len(fullMatches) >= 5 {
					break
				}

			} else if len(partialMatches) < 5 {
				if strings.Contains(strings.ToLower(v.User.Username), lowerIn) {
					partialMatches = append(partialMatches, v)
				}
			}
		}

		return true
	})

	if len(fullMatches) == 1 {
		return fullMatches[0], nil
	}

	if len(fullMatches) == 0 && len(partialMatches) == 0 {
		return nil, &UserNotFound{str}
	}

	out := ""
	for _, v := range fullMatches {
		if out != "" {
			out += ", "
		}

		out += "`" + v.User.Username + "`"
	}

	for _, v := range partialMatches {
		if out != "" {
			out += ", "
		}

		out += "`" + v.User.Username + "`"
	}

	if len(fullMatches) > 1 {
		return nil, NewSimpleUserError("Too many users with that name, " + out + ". Please re-run the command with a narrower search, mention or ID.")
	}

	return nil, NewSimpleUserError("Did you mean one of these? " + out + ". Please re-run the command with a narrower search, mention or ID")
}

// UserIDArg matches a mention or a plain id, the user does not have to be a part of the server
// The type of the ID is parsed into a int64
type UserIDArg struct{}

var _ ArgType = (*UserIDArg)(nil)

func (u *UserIDArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	// Check for mention
	if strings.HasPrefix(part, "<@") && strings.HasSuffix(part, ">") {
		return DetermineSnowflakeCompatibility(strings.TrimPrefix(part[2:len(part)-1], "!"))
	}

	// Check for ID
	return DetermineSnowflakeCompatibility(part)
}

func (u *UserIDArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {
	if strings.HasPrefix(part, "<@") && len(part) > 3 {
		// Direct mention
		id := part[2 : len(part)-1]
		if id[0] == '!' {
			// Nickname mention
			id = id[1:]
		}

		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, &ImproperMention{part}
		}

		return parsed, nil
	}

	id, err := strconv.ParseInt(part, 10, 64)
	if err == nil {
		return id, nil
	}

	return nil, &ImproperMention{part}
}

func (u *UserIDArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {
	user, found, err := options.ExpectUserOpt(def.Name)
	if err != nil {
		return nil, err
	}
	if found {
		return user.ID, nil
	}

	idStr, err := options.ExpectString(def.Name)
	if err != nil {
		return nil, err
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	return id, err
}

func (u *UserIDArg) HelpName() string {
	return "Mention/ID"
}

func (u *UserIDArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionUser)}
}

// UserIDArg matches a mention or a plain id, the user does not have to be a part of the server
// The type of the ID is parsed into a int64
type ChannelArg struct {
	AllowThreads bool
}

var _ ArgType = (*ChannelArg)(nil)

func (ca *ChannelArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	// Check for mention
	if strings.HasPrefix(part, "<#") && strings.HasSuffix(part, ">") {
		return DetermineSnowflakeCompatibility(part[2 : len(part)-1])
	}

	// Check for ID
	return DetermineSnowflakeCompatibility(part)
}

func (ca *ChannelArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {
	if data.GuildData == nil {
		return nil, nil
	}

	var cID int64
	if strings.HasPrefix(part, "<#") && len(part) > 3 {
		// Direct mention
		id := part[2 : len(part)-1]

		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, &ImproperMention{part}
		}

		cID = parsed
	} else {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, &ImproperMention{part}
		}
		cID = id
	}

	if ca.AllowThreads {
		if c := data.GuildData.GS.GetChannelOrThread(cID); c != nil {
			return c, nil
		}
	} else {
		if c := data.GuildData.GS.GetChannel(cID); c != nil {
			return c, nil
		}
	}

	return nil, &ImproperMention{part}
}

func (ca *ChannelArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {
	if data.GuildData == nil {
		return nil, nil
	}

	channel, err := options.ExpectChannel(def.Name)
	if err != nil {
		return nil, err
	}

	if ca.AllowThreads {
		if cs := data.GuildData.GS.GetChannelOrThread(channel.ID); cs != nil {
			return cs, nil
		}
	} else {
		if cs := data.GuildData.GS.GetChannel(channel.ID); cs != nil {
			return cs, nil
		}
	}

	return ErrChannelNotFound, nil
}

func (ca *ChannelArg) HelpName() string {
	return "Channel"
}

func (ca *ChannelArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionChannel)}
}

type AdvUserMatch struct {
	// Member may not be present if "RequireMembership" is false
	Member *dstate.MemberState

	// User is always present
	User *discordgo.User
}

func (a *AdvUserMatch) UsernameOrNickname() string {
	if a.Member != nil && a.Member.Member != nil {
		if a.Member.Member.Nick != "" {
			return a.Member.Member.Nick
		}
	}

	return a.User.Username
}

// AdvUserArg is a more advanced version of UserArg and UserIDArg, it will return a AdvUserMatch
type AdvUserArg struct {
	EnableUserID         bool // Whether to check for user IDS
	EnableUsernameSearch bool // Whether to search for usernames
	RequireMembership    bool // Whether this requires a membership of the server, if set then Member will always be populated
}

var _ ArgType = (*AdvUserArg)(nil)

func (u *AdvUserArg) CheckCompatibility(def *ArgDef, part string) CompatibilityResult {
	if strings.HasPrefix(part, "<@") && strings.HasSuffix(part, ">") {
		return DetermineSnowflakeCompatibility(strings.TrimPrefix(part[2:len(part)-1], "!"))
	}

	if u.EnableUserID {
		if DetermineSnowflakeCompatibility(part[2:len(part)-1]) == CompatibilityGood {
			return CompatibilityGood
		}
	}

	if u.EnableUsernameSearch {
		// username search
		return CompatibilityGood
	}

	return Incompatible
}

func (u *AdvUserArg) ParseFromMessage(def *ArgDef, part string, data *Data) (interface{}, error) {

	var user *discordgo.User
	var ms *dstate.MemberState

	// check mention
	if strings.HasPrefix(part, "<@") && len(part) > 3 {
		user = u.ParseMention(def, part, data)
	}

	msFailed := false
	if user == nil && u.EnableUserID {
		// didn't find a match in the previous step
		// try userID search
		if parsed, err := strconv.ParseInt(part, 10, 64); err == nil {
			ms, user = u.SearchID(parsed, data)
			if ms == nil {
				msFailed = true
			}
		}
	}

	if u.EnableUsernameSearch && data.GuildData != nil && ms == nil && user == nil {
		// Search for username
		var err error
		ms, err = FindDiscordMemberByName(data.System.State, data.GuildData.GS, part)
		if err != nil {
			return nil, err
		}
	}

	if ms == nil && user == nil {
		return nil, NewSimpleUserError("User/Member not found")
	}

	if ms != nil && user == nil {
		user = &ms.User
	} else if ms == nil && user != nil && !msFailed {
		ms, user = u.SearchID(user.ID, data)
	}

	return &AdvUserMatch{
		Member: ms,
		User:   user,
	}, nil
}

func (u *AdvUserArg) ParseFromInteraction(def *ArgDef, data *Data, options *SlashCommandsParseOptions) (val interface{}, err error) {
	user, found, err := options.ExpectUserOpt(def.Name)
	if err != nil {
		return nil, err
	}
	if found {
		// They used the user arg type
		member, err := options.ExpectMember(def.Name)
		if err != nil {
			return nil, err
		}

		return &AdvUserMatch{
			Member: dstate.MemberStateFromMember(member),
			User:   user,
		}, nil
	}

	// fall back by searching by ID
	id, err := options.ExpectInt64(def.Name + "-ID")
	if err != nil {
		return nil, err
	}

	ms, user := u.SearchID(id, data)
	return &AdvUserMatch{
		Member: ms,
		User:   user,
	}, nil
}

func (u *AdvUserArg) SearchID(parsed int64, data *Data) (member *dstate.MemberState, user *discordgo.User) {

	if data.GuildData != nil {
		// attempt to fetch member
		member = data.System.State.GetMember(data.GuildData.GS.ID, parsed)
		if member != nil {
			return member, &member.User
		}

		m, err := data.Session.GuildMember(data.GuildData.GS.ID, parsed)
		if err == nil {
			member = dstate.MemberStateFromMember(m)
			return member, m.User
		}
	}

	if u.RequireMembership {
		return nil, nil
	}

	// fallback to standard user
	user, _ = data.Session.User(parsed)
	return
}

func (u *AdvUserArg) ParseMention(def *ArgDef, part string, data *Data) (user *discordgo.User) {
	// Direct mention
	id := part[2 : len(part)-1]
	if id[0] == '!' {
		// Nickname mention
		id = id[1:]
	}

	parsed, _ := strconv.ParseInt(id, 10, 64)
	for _, v := range data.TraditionalTriggerData.Message.Mentions {
		if parsed == v.ID {
			return v
		}
	}

	return nil
}

func (u *AdvUserArg) HelpName() string {
	out := "User mention"
	if u.EnableUsernameSearch {
		out += "/Name"
	}
	if u.EnableUserID {
		out += "/ID"
	}

	return out
}

func (u *AdvUserArg) SlashCommandOptions(def *ArgDef) []*discordgo.ApplicationCommandOption {
	// Give the user the ability to pick one of these, sadly discord slash commands does not have a basic "one of" type
	optID := def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionInteger)
	optUser := def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionUser)

	optID.Name = optID.Name + "-ID"

	return []*discordgo.ApplicationCommandOption{optUser, optID}
}
