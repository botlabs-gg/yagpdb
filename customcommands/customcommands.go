package customcommands

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"emperror.dev/errors"
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

func (t CommandTriggerType) String() string {
	return triggerStrings[t]
}

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
	RedirectErrorsChannel int64 `schema:"redirect_errors_channel" valid:"channel,true"`

	TimeTriggerInterval       int     `schema:"time_trigger_interval"`
	TimeTriggerExcludingDays  []int64 `schema:"time_trigger_excluding_days"`
	TimeTriggerExcludingHours []int64 `schema:"time_trigger_excluding_hours"`

	ReactionTriggerMode  int `schema:"reaction_trigger_mode"`
	InteractionDeferMode int `schema:"interaction_defer_mode"`

	// If set, then the following channels are required, otherwise they are ignored
	RequireChannels bool    `json:"require_channels" schema:"require_channels"`
	Channels        []int64 `json:"channels" schema:"channels"`

	// If set, then one of the following channels are required, otherwise they are ignored
	RequireRoles  bool    `json:"require_roles" schema:"require_roles"`
	Roles         []int64 `json:"roles" schema:"roles"`
	TriggerOnEdit bool    `json:"trigger_on_edit" schema:"trigger_on_edit"`

	GroupID int64

	ShowErrors bool `schema:"show_errors"`
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

	return true
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
		if common.ContainsInt64Slice(ms.Member.Roles, v) {
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
	MaxCommands                 = 100
	MaxCommandsPremium          = 250
	MaxCCResponsesLength        = 10000
	MaxCCResponsesLengthPremium = 20000
	MaxUserMessages             = 20
	MaxGroups                   = 50
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
