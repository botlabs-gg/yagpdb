package customcommands

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/web"
	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"sort"
)

var (
	RegexCache *ccache.Cache
)

func KeyCommands(guildID int64) string { return "custom_commands:" + discordgo.StrID(guildID) }

type Plugin struct{}

func RegisterPlugin() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		log.WithError(err).Error("failed initializing custom commands schema, not enabling")
		return
	}

	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
	RegexCache = ccache.New(ccache.Configure())
}

func (p *Plugin) Name() string {
	return "Custom commands"
}

type CommandTriggerType int

const (
	CommandTriggerCommand CommandTriggerType = iota
	CommandTriggerStartsWith
	CommandTriggerContains
	CommandTriggerRegex
	CommandTriggerExact
)

var (
	triggerStrings = map[CommandTriggerType]string{
		CommandTriggerCommand:    "Command",
		CommandTriggerStartsWith: "StartsWith",
		CommandTriggerContains:   "Contains",
		CommandTriggerRegex:      "Regex",
		CommandTriggerExact:      "Exact",
	}
)

func (t CommandTriggerType) String() string {
	return triggerStrings[t]
}

type CustomCommand struct {
	TriggerType     CommandTriggerType `json:"trigger_type"`
	TriggerTypeForm string             `json:"-" schema:"type"`
	Trigger         string             `json:"trigger" schema:"trigger" valid:",1,1000"`
	// TODO: Retire the legacy Response field.
	Response      string   `json:"response,omitempty" schema:"response" valid:"template,5000"`
	Responses     []string `json:"responses" schema:"responses" valid:"template,5000"`
	CaseSensitive bool     `json:"case_sensitive" schema:"case_sensitive"`
	ID            int64    `json:"id"`

	// If set, then the following channels are required, otherwise they are ignored
	RequireChannels bool    `json:"require_channels" schema:"require_channels"`
	Channels        []int64 `json:"channels" schema:"channels"`

	// If set, then one of the following channels are required, otherwise they are ignored
	RequireRoles bool    `json:"require_roles" schema:"require_roles"`
	Roles        []int64 `json:"roles" schema:"roles"`

	GroupID int64
}

var _ web.CustomValidator = (*CustomCommand)(nil)

func (cc *CustomCommand) Validate(tmpl web.TemplateData) (ok bool) {
	if len(cc.Responses) > MaxUserMessages {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Too many responses, max %d", MaxUserMessages)))
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

		TimeTriggerExcludingDays:  []int64{},
		TimeTriggerExcludingHours: []int64{},

		Responses: cc.Responses,
	}

	if cc.GroupID != 0 {
		pqCommand.GroupID = null.Int64From(cc.GroupID)
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
			log.WithError(err).WithField("guild", guild).WithField("custom_command", k).Error("Failed decoding custom command")
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
