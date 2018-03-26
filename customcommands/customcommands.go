package customcommands

//go:generate esc -o assets_gen.go -pkg customcommands -ignore ".go" assets/

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs"
	"github.com/mediocregopher/radix.v2/redis"
	log "github.com/sirupsen/logrus"
	"sort"
	"strconv"
)

const (
	MaxCommands = 100
)

func KeyCommands(guildID string) string { return "custom_commands:" + guildID }

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
	docs.AddPage("Custom Commands", FSMustString(false, "/assets/help-page.md"), nil)
}

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(bot.RedisWrapper(HandleMessageCreate), eventsystem.EventMessageCreate)
	commands.AddRootCommands(cmdListCommands)
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
	Response        string             `json:"response" schema:"response" valid:",3000"`
	CaseSensitive   bool               `json:"case_sensitive" schema:"case_sensitive"`
	ID              int                `json:"id"`

	// If set, then the following channels are required, otherwise they are ignored
	RequireChannels bool    `json:"require_channels" schema:"require_channels"`
	Channels        []int64 `json:"channels" schema:"channels"`

	// If set, then one of the following channels are required, otherwise they are ignored
	RequireRoles bool    `json:"require_roles" schema:"require_roles"`
	Roles        []int64 `json:"roles" schema:"roles"`
}

func (cc *CustomCommand) Save(client *redis.Client, guildID string) error {
	serialized, err := json.Marshal(cc)
	if err != nil {
		return err
	}

	err = client.Cmd("HSET", KeyCommands(guildID), cc.ID, serialized).Err
	return err
}

func (cc *CustomCommand) RunsInChannel(channel string) bool {
	parsed, _ := strconv.ParseInt(channel, 10, 64)

	for _, v := range cc.Channels {
		if v == parsed {
			if cc.RequireChannels {
				return true
			}

			// Ignore the channel
			return false
		}
	}

	// Not found
	if cc.RequireChannels {
		return false
	}

	// Not in ignore list
	return true
}

func (cc *CustomCommand) RunsForUser(m *discordgo.Member) bool {

	if len(cc.Roles) == 0 {
		// Fast path
		if cc.RequireRoles {
			return false
		}

		return true
	}

	pRoles := make([]int64, len(m.Roles))
	for i, r := range m.Roles {
		pRoles[i], _ = strconv.ParseInt(r, 10, 64)
	}

	for _, v := range cc.Roles {
		if common.ContainsInt64Slice(pRoles, v) {
			if cc.RequireRoles {
				return true
			}

			return false
		}
	}

	// Not found
	if cc.RequireRoles {
		return false
	}

	return true
}

func GetCommands(client *redis.Client, guild string) ([]*CustomCommand, int, error) {
	hash, err := client.Cmd("HGETALL", "custom_commands:"+guild).Map()
	if err != nil {
		return nil, 0, err
	}

	highest := 0
	result := make([]*CustomCommand, len(hash))

	// Decode the commands, and also calculate the highest id
	i := 0
	for k, raw := range hash {
		var decoded *CustomCommand
		err = json.Unmarshal([]byte(raw), &decoded)
		if err != nil {
			log.WithError(err).WithField("guild", guild).WithField("custom_command", k).Error("Failed decoding custom command")
			result[i] = &CustomCommand{}
		} else {
			result[i] = decoded
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
