package customcommands

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/web"
	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var (
	RegexCache = *ccache.New(ccache.Configure())
	logger     = common.GetPluginLogger(&Plugin{})
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
	CommandTriggerReaction   CommandTriggerType = 6

	CommandTriggerInterval CommandTriggerType = 5
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
	}

	triggerStrings = map[CommandTriggerType]string{
		CommandTriggerCommand:    "Command",
		CommandTriggerStartsWith: "StartsWith",
		CommandTriggerContains:   "Contains",
		CommandTriggerRegex:      "Regex",
		CommandTriggerExact:      "Exact",
		CommandTriggerInterval:   "Interval",
		CommandTriggerReaction:   "Reaction",
	}
)

const (
	ReactionModeBoth       = 0
	ReactionModeAddOnly    = 1
	ReactionModeRemoveOnly = 2
)

func (t CommandTriggerType) String() string {
	return triggerStrings[t]
}

type CustomCommand struct {
	TriggerType     CommandTriggerType `json:"trigger_type"`
	TriggerTypeForm string             `json:"-" schema:"type"`
	Trigger         string             `json:"trigger" schema:"trigger" valid:",0,1000"`
	// TODO: Retire the legacy Response field.
	Response      string   `json:"response,omitempty" schema:"response" valid:"template,10000"`
	Responses     []string `json:"responses" schema:"responses" valid:"template,10000"`
	CaseSensitive bool     `json:"case_sensitive" schema:"case_sensitive"`
	ID            int64    `json:"id"`

	ContextChannel int64 `schema:"context_channel" valid:"channel,true"`

	TimeTriggerInterval       int     `schema:"time_trigger_interval"`
	TimeTriggerExcludingDays  []int64 `schema:"time_trigger_excluding_days"`
	TimeTriggerExcludingHours []int64 `schema:"time_trigger_excluding_hours"`

	ReactionTriggerMode int `schema:"reaction_trigger_mode"`

	// If set, then the following channels are required, otherwise they are ignored
	RequireChannels bool    `json:"require_channels" schema:"require_channels"`
	Channels        []int64 `json:"channels" schema:"channels"`

	// If set, then one of the following channels are required, otherwise they are ignored
	RequireRoles bool    `json:"require_roles" schema:"require_roles"`
	Roles        []int64 `json:"roles" schema:"roles"`

	GroupID int64

	ShowErrors bool `schema:"show_errors"`
}

var _ web.CustomValidator = (*CustomCommand)(nil)

func (cc *CustomCommand) Validate(tmpl web.TemplateData) (ok bool) {
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
		tmpl.AddAlerts(web.ErrorAlert("No response set"))
		return false
	}

	combinedSize := 0
	for _, v := range cc.Responses {
		combinedSize += utf8.RuneCountInString(v)
	}

	if combinedSize > 10000 {
		tmpl.AddAlerts(web.ErrorAlert("Max combined command size can be 10k"))
		return false
	}

	return true
}

func (cc *CustomCommand) ToDBModel() *models.CustomCommand {
	pqCommand := &models.CustomCommand{
		TriggerType:              int(cc.TriggerType),
		TextTrigger:              cc.Trigger,
		TextTriggerCaseSensitive: cc.CaseSensitive,

		Channels:              cc.Channels,
		ChannelsWhitelistMode: cc.RequireChannels,
		Roles:                 cc.Roles,
		RolesWhitelistMode:    cc.RequireRoles,

		TimeTriggerInterval:       cc.TimeTriggerInterval,
		TimeTriggerExcludingDays:  cc.TimeTriggerExcludingDays,
		TimeTriggerExcludingHours: cc.TimeTriggerExcludingHours,
		ContextChannel:            cc.ContextChannel,

		ReactionTriggerMode: int16(cc.ReactionTriggerMode),

		Responses: cc.Responses,

		ShowErrors: cc.ShowErrors,
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

	if cc.TriggerTypeForm == "interval_hours" {
		pqCommand.TimeTriggerInterval *= 60
	}

	return pqCommand
}

func CmdRunsInChannel(cc *models.CustomCommand, channel int64) bool {
	if cc.GroupID.Valid {
		// check group restrictions
		if common.ContainsInt64Slice(cc.R.Group.IgnoreChannels, channel) {
			return false
		}

		if len(cc.R.Group.WhitelistChannels) > 0 {
			if !common.ContainsInt64Slice(cc.R.Group.WhitelistChannels, channel) {
				return false
			}
		}
	}

	// check command specifc restrictions
	for _, v := range cc.Channels {
		if v == channel {
			if cc.ChannelsWhitelistMode {
				return true
			}

			// Ignore the channel
			return false
		}
	}

	// Not found
	if cc.ChannelsWhitelistMode {
		return false
	}

	// Not in ignore list
	return true
}

func CmdRunsForUser(cc *models.CustomCommand, ms *dstate.MemberState) bool {
	if cc.GroupID.Valid {
		// check group restrictions
		if common.ContainsInt64SliceOneOf(cc.R.Group.IgnoreRoles, ms.Roles) {
			return false
		}

		if len(cc.R.Group.WhitelistRoles) > 0 && !common.ContainsInt64SliceOneOf(cc.R.Group.WhitelistRoles, ms.Roles) {
			return false
		}
	}

	// check command specific restrictions
	if len(cc.Roles) == 0 {
		// Fast path
		if cc.RolesWhitelistMode {
			return false
		}

		return true
	}

	for _, v := range cc.Roles {
		if common.ContainsInt64Slice(ms.Roles, v) {
			if cc.RolesWhitelistMode {
				return true
			}

			return false
		}
	}

	// Not found
	if cc.RolesWhitelistMode {
		return false
	}

	return true
}

// Migrate modifies a CustomCommand to remove legacy fields.
func (cc *CustomCommand) Migrate() *CustomCommand {
	cc.Responses = filterEmptyResponses(cc.Response, cc.Responses...)
	cc.Response = ""
	if len(cc.Responses) > MaxUserMessages {
		cc.Responses = cc.Responses[:MaxUserMessages]
	}

	return cc
}

func LegacyGetCommands(guild int64) ([]*CustomCommand, int64, error) {
	var hashMap map[string]string

	err := common.RedisPool.Do(radix.Cmd(&hashMap, "HGETALL", KeyCommands(guild)))
	if err != nil {
		return nil, 0, err
	}

	highest := int64(0)
	result := make([]*CustomCommand, len(hashMap))

	// Decode the commands, and also calculate the highest id
	i := 0
	for k, raw := range hashMap {
		var decoded *CustomCommand
		err = json.Unmarshal([]byte(raw), &decoded)
		if err != nil {
			logger.WithError(err).WithField("guild", guild).WithField("custom_command", k).Error("Failed decoding custom command")
			result[i] = &CustomCommand{}
		} else {
			result[i] = decoded.Migrate()
			if decoded.ID > highest {
				highest = decoded.ID
			}
		}
		i++
	}

	// Sort by id
	sort.Sort(CustomCommandSlice(result))

	return result, highest, nil
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

func filterEmptyResponses(s string, ss ...string) []string {
	result := make([]string, 0, len(ss)+1)
	if s != "" {
		result = append(result, s)
	}

	for _, s := range ss {
		if s != "" {
			result = append(result, s)
		}
	}

	return result
}

const (
	MaxCommands        = 100
	MaxCommandsPremium = 250
	MaxUserMessages    = 20
	MaxGroups          = 50
)

func MaxCommandsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return MaxCommandsPremium
	}

	return MaxCommands
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
