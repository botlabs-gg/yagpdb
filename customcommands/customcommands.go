package customcommands

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/karlseguin/ccache"
	"github.com/robfig/cron/v3"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var (
	RegexCache = *ccache.New(ccache.Configure())
	logger     = common.GetPluginLogger(&Plugin{})
)

// Setting it to 1 Month approx
const (
	MinIntervalTriggerDurationMinutes = 5
	MinIntervalTriggerDurationHours   = 1
	MaxIntervalTriggerDurationHours   = 744
	MaxIntervalTriggerDurationMinutes = 44640

	dbPageMaxDisplayLength = 64
)

func KeyCommands(guildID int64) string { return "custom_commands:" + discordgo.StrID(guildID) }

type Plugin struct{}

func RegisterPlugin() {
	common.InitSchemas("customcommands", DBSchemas...)

	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Custom Commands",
		SysName:  "custom_commands",
		Category: common.PluginCategoryCore,
	}
}

type CommandTriggerType int

const (
	// The ordering of these might seem weird, but they're used in a database so changes would require migrations of a lot of data
	// yeah... i wish i was smarter when i made this originally

	CommandTriggerNone CommandTriggerType = 10

	CommandTriggerCommand    CommandTriggerType = 0
	CommandTriggerStartsWith CommandTriggerType = 1
	CommandTriggerContains   CommandTriggerType = 2
	CommandTriggerRegex      CommandTriggerType = 3
	CommandTriggerExact      CommandTriggerType = 4
	CommandTriggerInterval   CommandTriggerType = 5
	CommandTriggerReaction   CommandTriggerType = 6
	CommandTriggerComponent  CommandTriggerType = 7
	CommandTriggerModal      CommandTriggerType = 8
	CommandTriggerCron       CommandTriggerType = 9
	CommandTriggerRole       CommandTriggerType = 11
	CommandTriggerSlash      CommandTriggerType = 12
)

var (
	AllTriggerTypes = []CommandTriggerType{
		CommandTriggerCommand,
		CommandTriggerStartsWith,
		CommandTriggerContains,
		CommandTriggerRegex,
		CommandTriggerExact,
		CommandTriggerInterval,
		CommandTriggerReaction,
		CommandTriggerNone,
		CommandTriggerComponent,
		CommandTriggerModal,
		CommandTriggerCron,
		CommandTriggerRole,
		CommandTriggerSlash,
	}

	triggerStrings = map[CommandTriggerType]string{
		CommandTriggerCommand:    "Command",
		CommandTriggerStartsWith: "StartsWith",
		CommandTriggerContains:   "Contains",
		CommandTriggerRegex:      "Regex",
		CommandTriggerExact:      "Exact",
		CommandTriggerInterval:   "Interval",
		CommandTriggerReaction:   "Reaction",
		CommandTriggerNone:       "None",
		CommandTriggerComponent:  "Component",
		CommandTriggerModal:      "Modal",
		CommandTriggerCron:       "Crontab",
		CommandTriggerRole:       "Role",
		CommandTriggerSlash:      "Slash Command",
	}
)

const (
	ReactionModeBoth = iota
	ReactionModeAddOnly
	ReactionModeRemoveOnly
)

const (
	InteractionDeferModeNone = iota
	InteractionDeferModeMessage
	InteractionDeferModeEphemeral
	InteractionDeferModeUpdate
)

const (
	RoleTriggerModeBoth = iota
	RoleTriggerModeAdd
	RoleTriggerModeRemove
)

func (t CommandTriggerType) String() string {
	return triggerStrings[t]
}

type SlashCommandOption struct {
	Name        string `json:"name"`
	Type        int    `json:"type"` // discordgo.ApplicationCommandOptionType (3,4,5,6,7,8,9,10)
	Description string `json:"description"`
	Required    bool   `json:"required"`

	Choices []string `json:"choices,omitempty"`

	MinValue *float64 `json:"min_value,omitempty"`
	MaxValue *float64 `json:"max_value,omitempty"`

	MinLength *int `json:"min_length,omitempty"`
	MaxLength *int `json:"max_length,omitempty"`

	ChannelTypes []int `json:"channel_types,omitempty"`
}

// SlashCommandSubcommand is one subcommand of a slash command custom command. When a
// command has subcommands it has no top-level options and is invoked as
// "/<command> <subcommand>"; the invoked name is exposed to the template as .SubCommand.
type SlashCommandSubcommand struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Options     []SlashCommandOption `json:"options,omitempty"`
}

// slashCommandNameRegex matches Discord's allowed application command/option name
// pattern (lowercased). See https://discord.com/developers/docs/interactions/application-commands#application-command-object
var slashCommandNameRegex = regexp.MustCompile(`^[-_\p{L}\p{N}]{1,32}$`)

const (
	MaxSlashCommandOptions      = 25
	MaxSlashCommandDescription  = 100
	MaxSlashCommandChoices      = 25
	MaxSlashCommandChoiceLength = 100
	MaxSlashCommandStringLength = 6000
)

type CustomCommand struct {
	TriggerType     CommandTriggerType `json:"trigger_type"`
	TriggerTypeForm string             `json:"-" schema:"type"`
	Trigger         string             `json:"trigger" schema:"trigger" valid:",0,1000"`
	Responses       []string           `json:"responses" schema:"responses" valid:"template,20000"`
	CaseSensitive   bool               `json:"case_sensitive" schema:"case_sensitive"`
	ID              int64              `json:"id"`
	Name            string             `json:"name" schema:"name" valid:",0,100"`
	IsEnabled       bool               `json:"is_enabled" schema:"is_enabled"`
	Public          bool               `json:"public" schema:"public"`
	PublicID        string             `json:"public_id" schema:"public_id"`

	ContextChannel        int64 `schema:"context_channel" valid:"channel,true"`
	TimeContextChannel    int64 `schema:"time_context_channel" valid:"channel,true"`
	RoleContextChannel    int64 `schema:"role_context_channel" valid:"channel,true"`
	RedirectErrorsChannel int64 `schema:"redirect_errors_channel" valid:"channel,true"`

	TimeTriggerInterval       int     `schema:"time_trigger_interval"`
	TimeTriggerExcludingDays  []int64 `schema:"time_trigger_excluding_days"`
	TimeTriggerExcludingHours []int64 `schema:"time_trigger_excluding_hours"`

	ReactionTriggerMode  int `schema:"reaction_trigger_mode"`
	InteractionDeferMode int `schema:"interaction_defer_mode"`

	RoleTriggerMode int `schema:"role_trigger_mode"`

	// If set, then the following channels are required, otherwise they are ignored
	RequireChannels bool    `json:"require_channels" schema:"require_channels"`
	Channels        []int64 `json:"channels" schema:"channels"`

	// If set, then one of the following channels are required, otherwise they are ignored
	RequireRoles  bool    `json:"require_roles" schema:"require_roles"`
	Roles         []int64 `json:"roles" schema:"roles"`
	TriggerOnEdit bool    `json:"trigger_on_edit" schema:"trigger_on_edit"`

	GroupID int64

	ShowErrors bool `schema:"show_errors"`

	SlashCommandDescription string                   `schema:"slash_command_description" valid:",0,100"`
	SlashOptions            []SlashCommandOptionForm `schema:"slash_options"`
	// When SlashUseSubcommands is set the command has subcommands (slash_subcommands)
	// instead of top-level options; the two are mutually exclusive on Discord.
	SlashUseSubcommands bool                         `schema:"slash_use_subcommands"`
	SlashSubcommands    []SlashCommandSubcommandForm `schema:"slash_subcommands"`
}

type SlashCommandSubcommandForm struct {
	Name        string                   `schema:"name"`
	Description string                   `schema:"description"`
	Options     []SlashCommandOptionForm `schema:"options"`
}

func (sf SlashCommandSubcommandForm) isEmpty() bool {
	if strings.TrimSpace(sf.Name) != "" || strings.TrimSpace(sf.Description) != "" {
		return false
	}
	for _, o := range sf.Options {
		if !o.isEmpty() {
			return false
		}
	}
	return true
}

type SlashCommandOptionForm struct {
	Name string `schema:"name"`
	// Type is a UI key (e.g. "string", "string_menu", "integer", "integer_menu",
	// "number", "number_menu", "boolean", "user", "channel", "role", "mentionable").
	// The *_menu variants map to the same Discord type as their base but carry choices
	// instead of min/max constraints (the two are mutually exclusive on Discord).
	Type         string `schema:"type"`
	Description  string `schema:"description"`
	Required     bool   `schema:"required"`
	Choices      string `schema:"choices"`
	MinValue     string `schema:"min_value"`
	MaxValue     string `schema:"max_value"`
	MinLength    string `schema:"min_length"`
	MaxLength    string `schema:"max_length"`
	ChannelTypes []int  `schema:"channel_types"`
}

// slashFormType maps a slash option UI type key to its Discord option type and whether
// it's a "menu" (choices) variant. ok is false for unknown keys.
func slashFormType(key string) (discordType int, isMenu, ok bool) {
	switch key {
	case "string":
		return int(discordgo.ApplicationCommandOptionString), false, true
	case "string_menu":
		return int(discordgo.ApplicationCommandOptionString), true, true
	case "integer":
		return int(discordgo.ApplicationCommandOptionInteger), false, true
	case "integer_menu":
		return int(discordgo.ApplicationCommandOptionInteger), true, true
	case "number":
		return int(discordgo.ApplicationCommandOptionNumber), false, true
	case "number_menu":
		return int(discordgo.ApplicationCommandOptionNumber), true, true
	case "boolean":
		return int(discordgo.ApplicationCommandOptionBoolean), false, true
	case "user":
		return int(discordgo.ApplicationCommandOptionUser), false, true
	case "channel":
		return int(discordgo.ApplicationCommandOptionChannel), false, true
	case "role":
		return int(discordgo.ApplicationCommandOptionRole), false, true
	case "mentionable":
		return int(discordgo.ApplicationCommandOptionMentionable), false, true
	}
	return 0, false, false
}

// slashFormTypeKey is the inverse of slashFormType for rendering a stored option back
// into the editor's type dropdown. A *_menu key is returned when choices are present.
func slashFormTypeKey(opt SlashCommandOption) string {
	switch opt.Type {
	case int(discordgo.ApplicationCommandOptionString):
		if len(opt.Choices) > 0 {
			return "string_menu"
		}
		return "string"
	case int(discordgo.ApplicationCommandOptionInteger):
		if len(opt.Choices) > 0 {
			return "integer_menu"
		}
		return "integer"
	case int(discordgo.ApplicationCommandOptionNumber):
		if len(opt.Choices) > 0 {
			return "number_menu"
		}
		return "number"
	case int(discordgo.ApplicationCommandOptionBoolean):
		return "boolean"
	case int(discordgo.ApplicationCommandOptionUser):
		return "user"
	case int(discordgo.ApplicationCommandOptionChannel):
		return "channel"
	case int(discordgo.ApplicationCommandOptionRole):
		return "role"
	case int(discordgo.ApplicationCommandOptionMentionable):
		return "mentionable"
	}
	return "string"
}

func (f SlashCommandOptionForm) isEmpty() bool {
	return strings.TrimSpace(f.Name) == "" &&
		strings.TrimSpace(f.Description) == "" &&
		strings.TrimSpace(f.Choices) == "" &&
		strings.TrimSpace(f.MinValue) == "" &&
		strings.TrimSpace(f.MaxValue) == "" &&
		strings.TrimSpace(f.MinLength) == "" &&
		strings.TrimSpace(f.MaxLength) == "" &&
		len(f.ChannelTypes) == 0
}

func (cc *CustomCommand) SlashCommandOptions() []SlashCommandOption {
	return parseSlashOptionForms(cc.SlashOptions)
}

// parseSlashOptionForms converts a list of option form rows into resolved options,
// dropping empty rows and sorting required-first. Shared by the top-level options and
// per-subcommand options.
func parseSlashOptionForms(forms []SlashCommandOptionForm) []SlashCommandOption {
	opts := make([]SlashCommandOption, 0, len(forms))
	for _, f := range forms {
		if f.isEmpty() {
			continue
		}

		// resolve the UI type key into the Discord type and whether it's a choices menu.
		dType, isMenu, _ := slashFormType(f.Type)

		opt := SlashCommandOption{
			Name:        strings.TrimSpace(f.Name),
			Type:        dType,
			Description: f.Description,
			Required:    f.Required,
		}

		// menu variants carry choices; the plain variants carry min/max constraints.
		// the two are mutually exclusive on Discord, enforced here by the type key.
		switch {
		case isMenu:
			opt.Choices = parseChoiceLines(f.Choices)
		case dType == int(discordgo.ApplicationCommandOptionString):
			opt.MinLength = parseIntPtr(f.MinLength)
			opt.MaxLength = parseIntPtr(f.MaxLength)
		case dType == int(discordgo.ApplicationCommandOptionInteger), dType == int(discordgo.ApplicationCommandOptionNumber):
			opt.MinValue = parseFloatPtr(f.MinValue)
			opt.MaxValue = parseFloatPtr(f.MaxValue)
		case dType == int(discordgo.ApplicationCommandOptionChannel):
			for _, ct := range f.ChannelTypes {
				if validSlashChannelTypes[ct] {
					opt.ChannelTypes = append(opt.ChannelTypes, ct)
				}
			}
		}

		opts = append(opts, opt)
	}

	sort.SliceStable(opts, func(i, j int) bool {
		return opts[i].Required && !opts[j].Required
	})

	return opts
}

// SlashCommandSubcommands converts the subcommand form rows into resolved subcommands,
// dropping fully-empty rows.
func (cc *CustomCommand) SlashCommandSubcommands() []SlashCommandSubcommand {
	subs := make([]SlashCommandSubcommand, 0, len(cc.SlashSubcommands))
	for _, sf := range cc.SlashSubcommands {
		if sf.isEmpty() {
			continue
		}
		subs = append(subs, SlashCommandSubcommand{
			Name:        strings.TrimSpace(sf.Name),
			Description: sf.Description,
			Options:     parseSlashOptionForms(sf.Options),
		})
	}
	return subs
}

func parseChoiceLines(raw string) []string {
	var out []string
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseIntPtr(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if v, err := strconv.Atoi(raw); err == nil {
		return &v
	}
	return nil
}

func parseFloatPtr(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return &v
	}
	return nil
}

var _ web.CustomValidator = (*CustomCommand)(nil)

func validateCCResponseLength(responses []string, guild_id int64) bool {
	combinedSize := 0
	for _, v := range responses {
		combinedSize += utf8.RuneCountInString(v)
	}

	ccMaxLength := MaxCCResponsesLength
	isGuildPremium, _ := premium.IsGuildPremium(guild_id)
	if isGuildPremium {
		ccMaxLength = MaxCCResponsesLengthPremium
	}

	return combinedSize <= ccMaxLength
}

func (cc *CustomCommand) Validate(tmpl web.TemplateData, guild_id int64) (ok bool) {
	if len(cc.Responses) > MaxUserMessages {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Too many responses, max %d", MaxUserMessages)))
		return false
	}

	foundOkayResponse := false
	for _, v := range cc.Responses {
		if strings.TrimSpace(v) != "" {
			foundOkayResponse = true
			break
		}
	}

	if !foundOkayResponse {
		if len(tmpl.Alerts()) == 0 {
			tmpl.AddAlerts(web.ErrorAlert("No response set"))
		}
		return false
	}

	isValidCCLength := validateCCResponseLength(cc.Responses, guild_id)

	if cc.IsEnabled && !isValidCCLength {
		tmpl.AddAlerts(web.ErrorAlert("Max combined command size can be 10k for free servers, and 20k for premium servers"))
		return false
	}

	if cc.TriggerTypeForm == "interval_minutes" && (cc.TimeTriggerInterval < MinIntervalTriggerDurationMinutes || cc.TimeTriggerInterval > MaxIntervalTriggerDurationMinutes) {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Minute interval can be between %v and %v", MinIntervalTriggerDurationMinutes, MaxIntervalTriggerDurationMinutes)))
		return false
	}

	if cc.TriggerTypeForm == "interval_hours" && (cc.TimeTriggerInterval < MinIntervalTriggerDurationHours || cc.TimeTriggerInterval > MaxIntervalTriggerDurationHours) {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Hourly interval can be between %v and %v", MinIntervalTriggerDurationHours, MaxIntervalTriggerDurationHours)))
		return false
	}

	if cc.TriggerTypeForm == "cron" {
		if _, err := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(cc.Trigger); err != nil {
			tmpl.AddAlerts(web.ErrorAlert("Error parsing cron spec: ", err.Error()))
			return false
		}
	}

	if cc.TriggerTypeForm == "role_trigger" {
		if cc.RoleTriggerMode < 0 || cc.RoleTriggerMode > 2 {
			tmpl.AddAlerts(web.ErrorAlert("Invalid role trigger mode"))
			return false
		}
	}

	if cc.TriggerTypeForm == "slash_command" {
		if !cc.validateSlashCommand(tmpl, guild_id) {
			return false
		}
	}

	return true

}

var validSlashOptionTypes = map[int]bool{
	int(discordgo.ApplicationCommandOptionString):      true,
	int(discordgo.ApplicationCommandOptionInteger):     true,
	int(discordgo.ApplicationCommandOptionBoolean):     true,
	int(discordgo.ApplicationCommandOptionUser):        true,
	int(discordgo.ApplicationCommandOptionChannel):     true,
	int(discordgo.ApplicationCommandOptionRole):        true,
	int(discordgo.ApplicationCommandOptionMentionable): true,
	int(discordgo.ApplicationCommandOptionNumber):      true,
}

// validSlashChannelTypes are the discordgo.ChannelType values that may be used to
// restrict a Channel option (guild channel kinds only).
var validSlashChannelTypes = map[int]bool{
	int(discordgo.ChannelTypeGuildText):          true,
	int(discordgo.ChannelTypeGuildVoice):         true,
	int(discordgo.ChannelTypeGuildCategory):      true,
	int(discordgo.ChannelTypeGuildNews):          true,
	int(discordgo.ChannelTypeGuildNewsThread):    true,
	int(discordgo.ChannelTypeGuildPublicThread):  true,
	int(discordgo.ChannelTypeGuildPrivateThread): true,
	int(discordgo.ChannelTypeGuildStageVoice):    true,
	int(discordgo.ChannelTypeGuildForum):         true,
}

func (cc *CustomCommand) validateSlashCommand(tmpl web.TemplateData, guildID int64) bool {
	if cc.Trigger != strings.ToLower(cc.Trigger) {
		tmpl.AddAlerts(web.ErrorAlert("Slash command name must be lowercase"))
		return false
	}

	if cc.InteractionDeferMode == InteractionDeferModeUpdate {
		tmpl.AddAlerts(web.ErrorAlert("\"Update message\" defer mode is not valid for slash commands"))
		return false
	}

	data := slashCommandData{Description: cc.SlashCommandDescription}
	if cc.SlashUseSubcommands {
		data.Subcommands = cc.SlashCommandSubcommands()
		if len(data.Subcommands) == 0 {
			tmpl.AddAlerts(web.ErrorAlert("Add at least one subcommand (with a name and description) or turn off the \"Use subcommands\" toggle"))
			return false
		}
	} else {
		data.Options = cc.SlashCommandOptions()
	}

	if ok, msg := validateSlashCommandData(guildID, cc.Trigger, data, cc.ID, cc.IsEnabled); !ok {
		tmpl.AddAlerts(web.ErrorAlert(msg))
		return false
	}

	return true
}

// validateSlashCommandData enforces Discord's slash command rules and the per-guild
// name-uniqueness and count limits. It is shared by the editor form and the import
// path so both reject invalid/duplicate commands. currentLocalID is the local_id of
// the command being saved (0 for new commands) and is excluded from the duplicate
// and limit checks.
func validateSlashCommandData(guildID int64, name string, data slashCommandData, currentLocalID int64, isEnabled bool) (ok bool, errMsg string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !slashCommandNameRegex.MatchString(name) {
		return false, "Slash command name must be 1-32 characters and contain only letters, numbers, dashes and underscores"
	}

	if l := utf8.RuneCountInString(data.Description); l < 1 || l > MaxSlashCommandDescription {
		return false, fmt.Sprintf("Slash command description must be between 1 and %d characters", MaxSlashCommandDescription)
	}

	if commands.IsInbuiltSlashCommandName(name) {
		return false, "A built-in command with that name already exists, please choose a different name"
	}

	if len(data.Subcommands) > 0 {
		// A command with subcommands is invoked as "/<command> <subcommand>" and has no
		// top-level options; subcommands reuse the same 3-free / 10-premium limit.
		maxSubs := MaxSlashCommandForContext(guildID)
		if len(data.Subcommands) > maxSubs {
			return false, fmt.Sprintf("You can have at most %d subcommands per command (%d on premium servers)", maxSubs, MaxSlashCommandCCsPremium)
		}

		seenSubs := make(map[string]bool, len(data.Subcommands))
		for _, sub := range data.Subcommands {
			sname := strings.TrimSpace(sub.Name)
			if !slashCommandNameRegex.MatchString(sname) {
				return false, fmt.Sprintf("Subcommand name %q must be 1-32 characters (letters, numbers, dashes, underscores)", sub.Name)
			}
			if sname != strings.ToLower(sname) {
				return false, fmt.Sprintf("Subcommand name %q must be lowercase", sub.Name)
			}
			if seenSubs[sname] {
				return false, fmt.Sprintf("Duplicate subcommand name %q", sname)
			}
			seenSubs[sname] = true
			if l := utf8.RuneCountInString(sub.Description); l < 1 || l > MaxSlashCommandDescription {
				return false, fmt.Sprintf("Description for subcommand %q must be between 1 and %d characters", sname, MaxSlashCommandDescription)
			}
			if ok, msg := validateSlashOptionList(sub.Options); !ok {
				return false, fmt.Sprintf("Subcommand %q: %s", sname, msg)
			}
		}
	} else if ok, msg := validateSlashOptionList(data.Options); !ok {
		return false, msg
	}

	existing, err := models.CustomCommands(qm.Where("guild_id = ? AND trigger_type = ? AND local_id != ?", guildID, int(CommandTriggerSlash), currentLocalID)).AllG(context.Background())
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed fetching existing slash command ccs for validation")
		return false, "Failed validating slash command, please try again"
	}

	enabledCount := 0
	for _, e := range existing {
		if strings.EqualFold(e.TextTrigger, name) {
			return false, "Another slash command in this server already uses that name"
		}
		if !e.Disabled {
			enabledCount++
		}
	}

	maxSlash := MaxSlashCommandForContext(guildID)
	if isEnabled && enabledCount >= maxSlash {
		return false, fmt.Sprintf("You can have at most %d enabled slash commands per server (%d on premium servers)", maxSlash, MaxSlashCommandCCsPremium)
	}

	return true, ""
}

// validateSlashOptionList validates a single option list (the command's top-level
// options, or one subcommand's options). Error messages reference the option name.
func validateSlashOptionList(options []SlashCommandOption) (ok bool, errMsg string) {
	if len(options) > MaxSlashCommandOptions {
		return false, fmt.Sprintf("can have at most %d options", MaxSlashCommandOptions)
	}

	seenOptions := make(map[string]bool, len(options))
	for _, opt := range options {
		oname := strings.TrimSpace(opt.Name)
		if !slashCommandNameRegex.MatchString(oname) {
			return false, fmt.Sprintf("Option name %q must be 1-32 characters (letters, numbers, dashes, underscores)", opt.Name)
		}

		if oname != strings.ToLower(oname) {
			return false, fmt.Sprintf("Option name %q must be lowercase", opt.Name)
		}
		if seenOptions[oname] {
			return false, fmt.Sprintf("Duplicate option name %q", oname)
		}
		seenOptions[oname] = true

		if l := utf8.RuneCountInString(opt.Description); l < 1 || l > MaxSlashCommandDescription {
			return false, fmt.Sprintf("Description for option %q must be between 1 and %d characters", oname, MaxSlashCommandDescription)
		}
		if !validSlashOptionTypes[opt.Type] {
			return false, fmt.Sprintf("Invalid type for option %q", oname)
		}

		if len(opt.Choices) > MaxSlashCommandChoices {
			return false, fmt.Sprintf("Option %q can have at most %d choices", oname, MaxSlashCommandChoices)
		}
		for _, c := range opt.Choices {
			if strings.TrimSpace(c) == "" {
				return false, fmt.Sprintf("Option %q has an empty choice", oname)
			}
			if utf8.RuneCountInString(c) > MaxSlashCommandChoiceLength {
				return false, fmt.Sprintf("Choice %q on option %q must be at most %d characters", c, oname, MaxSlashCommandChoiceLength)
			}
			// Integer/Number choice values must parse to the matching numeric type.
			if opt.Type == int(discordgo.ApplicationCommandOptionInteger) {
				if _, err := strconv.ParseInt(strings.TrimSpace(c), 10, 64); err != nil {
					return false, fmt.Sprintf("Choice %q on option %q must be a whole number", c, oname)
				}
			} else if opt.Type == int(discordgo.ApplicationCommandOptionNumber) {
				if _, err := strconv.ParseFloat(strings.TrimSpace(c), 64); err != nil {
					return false, fmt.Sprintf("Choice %q on option %q must be a number", c, oname)
				}
			}
		}

		if opt.MinValue != nil && opt.MaxValue != nil && *opt.MinValue > *opt.MaxValue {
			return false, fmt.Sprintf("Min value cannot be greater than max value for option %q", oname)
		}
		if opt.MinLength != nil && opt.MaxLength != nil && *opt.MinLength > *opt.MaxLength {
			return false, fmt.Sprintf("Min length cannot be greater than max length for option %q", oname)
		}
		if (opt.MinLength != nil && *opt.MinLength < 0) || (opt.MaxLength != nil && *opt.MaxLength > MaxSlashCommandStringLength) {
			return false, fmt.Sprintf("Length constraints for option %q must be between 0 and %d", oname, MaxSlashCommandStringLength)
		}
	}

	return true, ""
}

func (cc *CustomCommand) ToDBModel() *models.CustomCommand {
	pqCommand := &models.CustomCommand{
		TriggerType:              int(cc.TriggerType),
		TextTrigger:              cc.Trigger,
		TextTriggerCaseSensitive: cc.CaseSensitive,
		Public:                   cc.Public,
		PublicID:                 cc.PublicID,

		Channels:              cc.Channels,
		ChannelsWhitelistMode: cc.RequireChannels,
		Roles:                 cc.Roles,
		RolesWhitelistMode:    cc.RequireRoles,

		TimeTriggerInterval:       cc.TimeTriggerInterval,
		TimeTriggerExcludingDays:  cc.TimeTriggerExcludingDays,
		TimeTriggerExcludingHours: cc.TimeTriggerExcludingHours,
		ContextChannel:            cc.ContextChannel,
		RedirectErrorsChannel:     cc.RedirectErrorsChannel,

		ReactionTriggerMode:  int16(cc.ReactionTriggerMode),
		InteractionDeferMode: int16(cc.InteractionDeferMode),

		RoleTriggerMode: int16(cc.RoleTriggerMode),

		Responses: cc.Responses,

		ShowErrors:    cc.ShowErrors,
		Disabled:      !cc.IsEnabled,
		TriggerOnEdit: cc.TriggerOnEdit,
	}

	if cc.TimeTriggerExcludingDays == nil {
		pqCommand.TimeTriggerExcludingDays = []int64{}
	}

	if cc.TimeTriggerExcludingHours == nil {
		pqCommand.TimeTriggerExcludingHours = []int64{}
	}

	if cc.GroupID != 0 {
		pqCommand.GroupID = null.Int64From(cc.GroupID)
	}

	if cc.Name != "" {
		pqCommand.Name = null.StringFrom(cc.Name)
	} else {
		pqCommand.Name = null.NewString("", false)
	}

	if cc.TriggerTypeForm == "interval_hours" {
		pqCommand.TimeTriggerInterval *= 60
	}

	if cc.TriggerTypeForm == "slash_command" {
		pqCommand.TextTrigger = strings.ToLower(cc.Trigger)

		payload := slashCommandData{Description: cc.SlashCommandDescription}
		// subcommands and top-level options are mutually exclusive on Discord.
		if cc.SlashUseSubcommands {
			payload.Subcommands = cc.SlashCommandSubcommands()
		} else {
			payload.Options = cc.SlashCommandOptions()
		}
		if b, err := json.Marshal(payload); err == nil {
			pqCommand.SlashCommandOptions = null.JSONFrom(b)
		} else {
			logger.WithError(err).Error("failed marshalling slash command options")
		}
	}

	return pqCommand
}

// slashCommandData is the structure stored in the slash_command_options jsonb
// column for slash command custom commands. Options and Subcommands are mutually
// exclusive: a command with subcommands has no top-level options.
type slashCommandData struct {
	Description string                   `json:"description"`
	Options     []SlashCommandOption     `json:"options,omitempty"`
	Subcommands []SlashCommandSubcommand `json:"subcommands,omitempty"`
}

// parseSlashCommandData decodes the slash_command_options jsonb column of a stored
// custom command. It returns zero values if the column is null or invalid.
func parseSlashCommandData(cc *models.CustomCommand) slashCommandData {
	var data slashCommandData
	if !cc.SlashCommandOptions.Valid {
		return data
	}
	if err := json.Unmarshal(cc.SlashCommandOptions.JSON, &data); err != nil {
		logger.WithError(err).WithField("cc_id", cc.LocalID).Error("failed unmarshalling slash command options")
	}
	return data
}

func CmdRunsInChannel(cc *models.CustomCommand, channel int64) bool {
	if cc.GroupID.Valid {
		// check group restrictions
		if slices.Contains(cc.R.Group.IgnoreChannels, channel) {
			return false
		}

		if len(cc.R.Group.WhitelistChannels) > 0 {
			if !slices.Contains(cc.R.Group.WhitelistChannels, channel) {
				return false
			}
		}
	}

	// check command specifc restrictions
	for _, v := range cc.Channels {
		if v == channel {
			return cc.ChannelsWhitelistMode
		}
	}

	// Not found
	return !cc.ChannelsWhitelistMode
}

func CmdRunsForUser(cc *models.CustomCommand, ms *dstate.MemberState) bool {
	if cc.GroupID.Valid {
		// check group restrictions
		if common.ContainsInt64SliceOneOf(cc.R.Group.IgnoreRoles, ms.Member.Roles) {
			return false
		}

		if len(cc.R.Group.WhitelistRoles) > 0 && !common.ContainsInt64SliceOneOf(cc.R.Group.WhitelistRoles, ms.Member.Roles) {
			return false
		}
	}

	// check command specific restrictions
	if len(cc.Roles) == 0 {
		// Fast path
		return !cc.RolesWhitelistMode
	}

	for _, v := range cc.Roles {
		if slices.Contains(ms.Member.Roles, v) {
			return cc.RolesWhitelistMode
		}
	}

	// Not found
	return !cc.RolesWhitelistMode
}

type CustomCommandSlice []*CustomCommand

// Len is the number of elements in the collection.
func (c CustomCommandSlice) Len() int {
	return len(c)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (c CustomCommandSlice) Less(i, j int) bool {
	return c[i].ID < c[j].ID
}

// Swap swaps the elements with indexes i and j.
func (c CustomCommandSlice) Swap(i, j int) {
	temp := c[i]
	c[i] = c[j]
	c[j] = temp
}

const (
	MaxCommands                   = 100
	MaxCommandsPremium            = 500
	MaxRoleTriggerCommands        = 1
	MaxRoleTriggerCommandsPremium = 5
	MaxCCResponsesLength          = 10000
	MaxCCResponsesLengthPremium   = 20000
	MaxUserMessages               = 20
	MaxGroups                     = 50
	MaxSlashCommandCCs            = 3
	MaxSlashCommandCCsPremium     = 10
)

// MaxSlashCommandForContext returns how many enabled slash command custom commands
// a guild may have, depending on its premium status.
func MaxSlashCommandForContext(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return MaxSlashCommandCCsPremium
	}
	return MaxSlashCommandCCs
}

func MaxCommandsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return MaxCommandsPremium
	}

	return MaxCommands
}

func MaxRoleTriggerCommandsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return MaxRoleTriggerCommandsPremium
	}

	return MaxRoleTriggerCommands
}

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagHasCommands = "custom_commands_has_commands"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {

	var flags []string
	count, err := models.CustomCommands(qm.Where("guild_id = ?", guildID)).CountG(context.Background())
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	if count > 0 {
		flags = append(flags, featureFlagHasCommands)
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagHasCommands, // set if this server has any custom commands at all
	}
}

func getDatabaseEntries(ctx context.Context, guildID int64, page int, queryType, query string, limit int) (models.TemplatesUserDatabaseSlice, int64, error) {
	qms := []qm.QueryMod{
		models.TemplatesUserDatabaseWhere.GuildID.EQ(guildID),
	}

	if len(query) > 0 {
		switch queryType {
		case "id":
			qms = append(qms, qm.Where("id = ?", query))
		case "user_id":
			qms = append(qms, qm.Where("user_id = ?", query))
		case "key":
			qms = append(qms, qm.Where("key ILIKE ?", query))
		}
	}
	qms = append(qms, qm.Where("(expires_at IS NULL or expires_at > now())"))
	count, err := models.TemplatesUserDatabases(qms...).CountG(ctx)
	if int64(page) > (count / 100) {
		page = int(math.Ceil(float64(count) / 100))
	}
	if page > 1 {
		qms = append(qms, qm.Offset((limit * (page - 1))))
	}
	qms = append(qms, qm.OrderBy("id desc"), qm.Limit(limit))
	entries, err := models.TemplatesUserDatabases(qms...).AllG(ctx)
	return entries, count, err
}

func convertEntries(result models.TemplatesUserDatabaseSlice) []*LightDBEntry {
	entries := make([]*LightDBEntry, 0, len(result))
	for _, v := range result {
		converted, err := ToLightDBEntry(v)
		if err != nil {
			logger.WithError(err).Warn("[cc/web] failed converting to light db entry")
			continue
		}

		b, err := json.Marshal(converted.Value)
		if err != nil {
			logger.WithError(err).Warn("[cc/web] failed converting to light db entry")
			b = []byte("Failed to convert db entry to a readable format")
		}

		converted.Value = common.CutStringShort(string(b), dbPageMaxDisplayLength)

		entries = append(entries, converted)
	}

	return entries
}
