package customcommands

import (
	"encoding/json"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"log"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)
}

func (p *Plugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/custom_commands.html"))

	cpMux.HandleFuncC(pat.Get("/cp/:server/customcommands"), HandleCommands)
	cpMux.HandleFuncC(pat.Get("/cp/:server/customcommands/"), HandleCommands)

	// If only html allowed patch and delete.. if only
	cpMux.HandleFuncC(pat.Post("/cp/:server/customcommands"), HandleNewCommand)
	cpMux.HandleFuncC(pat.Post("/cp/:server/customcommands/:cmd/update"), HandleUpdateCommand)
	cpMux.HandleFuncC(pat.Post("/cp/:server/customcommands/:cmd/delete"), HandleDeleteCommand)
}

func (p *Plugin) InitBot() {
	bot.Session.AddHandler(HandleMessageCreate)
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
	TriggerType   CommandTriggerType `json:"trigger_type"`
	Trigger       string             `json:"trigger"`
	Response      string             `json:"response"`
	ID            int                `json:"id"`
	CaseSensitive bool               `json:"case_sensitive"`
}

func GetCommands(client *redis.Client, guild string) ([]*CustomCommand, int, error) {
	reply := client.Cmd("SELECT", 0)
	if reply.Err != nil {
		return nil, 0, reply.Err
	}

	hash, err := client.Cmd("HGETALL", "custom_commands:"+guild).Hash()
	if err != nil {
		if _, ok := err.(*redis.CmdError); ok {
			return []*CustomCommand{}, 0, nil
		} else {
			return nil, 0, err
		}
	}
	highest := 0
	result := make([]*CustomCommand, len(hash))
	i := 0
	for k, raw := range hash {
		var decoded *CustomCommand
		err = json.Unmarshal([]byte(raw), &decoded)
		if err != nil {
			log.Println("Failure decoding custom command", k, guild, err)
			result[i] = &CustomCommand{}
		} else {
			result[i] = decoded
			if decoded.ID > highest {
				highest = decoded.ID
			}
		}
		i++
	}

	return result, highest, nil
}
