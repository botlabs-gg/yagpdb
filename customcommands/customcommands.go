package customcommands

//go:generate esc -o assets_gen.go -pkg customcommands -ignore ".go" assets/

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/docs"
	"github.com/jonas747/yagpdb/web"
	"sort"
)

func KeyCommands(guildID string) string { return "custom_commands:" + guildID }

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)
	docs.AddPage("Custom Commands", FSMustString(false, "assets/help-page.md"), nil)
}

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(bot.RedisWrapper(HandleMessageCreate), eventsystem.EventMessageCreate)
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

type CustomCommand struct {
	TriggerType     CommandTriggerType `json:"trigger_type"`
	TriggerTypeForm string             `json:"-" schema:"type"`
	Trigger         string             `json:"trigger" schema:"trigger" valid:",1,2000"`
	Response        string             `json:"response" schema:"response" valid:",2000"`
	CaseSensitive   bool               `json:"case_sensitive" schema:"case_sensitive"`
	ID              int                `json:"id"`
}

func (cc *CustomCommand) Save(client *redis.Client, guildID string) error {
	serialized, err := json.Marshal(cc)
	if err != nil {
		return err
	}

	err = client.Cmd("HSET", KeyCommands(guildID), cc.ID, serialized).Err
	return err
}

func GetCommands(client *redis.Client, guild string) ([]*CustomCommand, int, error) {
	hash, err := client.Cmd("HGETALL", "custom_commands:"+guild).Hash()
	if err != nil {
		// Check if the error was that it didnt exist, if so return an empty slice
		// If not, there was an actual error
		if _, ok := err.(*redis.CmdError); ok {
			return []*CustomCommand{}, 0, nil
		} else {
			return nil, 0, err
		}
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
