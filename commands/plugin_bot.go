package commands

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/lunixbochs/vtclean"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// var (
// 	EscapeSquenceRegex = regexp.MustCompile(`(\x1b\[|\x9b)[^@-_]*[@-_]|\x1b[@-_]`)
// )

func (p *Plugin) InitBot() {
	bot.CommandSystem.Prefix = p
	bot.CommandSystem.RegisterCommands(GlobalCommands...)
}

func (p *Plugin) GetPrefix(s *discordgo.Session, m *discordgo.MessageCreate) string {
	client, err := bot.RedisPool.Get()
	if err != nil {
		log.Println("Failed redis connection from pool", err)
		return ""
	}
	defer bot.RedisPool.Put(client)

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.Println("Failed retrieving channel from state", err)
		return ""
	}

	config := GetConfig(client, channel.GuildID)
	return config.Prefix
}

var GlobalCommands = []commandsystem.CommandHandler{
	&commandsystem.SimpleCommand{
		Name:        "Help",
		Description: "Shows help abut all or one specific command",
		RunInDm:     true,
		Arguments: []*commandsystem.ArgumentDef{
			&commandsystem.ArgumentDef{Name: "command", Type: commandsystem.ArgumentTypeString},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			target := ""
			if parsed.Args[0] != nil {
				target = parsed.Args[0].Str()
			}
			help := bot.CommandSystem.GenerateHelp(target, 0)
			bot.Session.ChannelMessageSend(m.ChannelID, help)
			return nil
		},
	},
	// Status command shows the bot's status, stuff like version, conntected servers, uptime, memory used etc..
	&commandsystem.SimpleCommand{
		Name:        "Status",
		Description: "Shows yagpdb status",
		RunInDm:     true,
		RunFunc: func(cmd *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			servers := len(bot.Session.State.Guilds)

			uptime := time.Since(bot.Started)

			// Convert to megabytes for ez readings
			allocated := float64(memStats.Alloc) / 1000000
			totalAllocated := float64(memStats.TotalAlloc) / 1000000

			numGoroutines := runtime.NumGoroutine()

			status := fmt.Sprintf("**YAGPDB STATUS** *bot version: %s*\n - Connected Servers: %d\n - Uptime: %s\n - Allocated: %.2fMB\n - Total Allocated: %.2fMB\n - Number of Goroutines: %d\n",
				bot.VERSION, servers, uptime.String(), allocated, totalAllocated, numGoroutines)

			bot.Session.ChannelMessageSend(m.ChannelID, status)

			return nil
		},
	},
	// Some fun commands because why not
	&commandsystem.SimpleCommand{
		Name:         "Reverse",
		Aliases:      []string{"r", "rev"},
		Description:  "Flips stuff",
		RunInDm:      true,
		RequiredArgs: 1,
		Arguments: []*commandsystem.ArgumentDef{
			&commandsystem.ArgumentDef{Name: "What", Description: "To flip", Type: commandsystem.ArgumentTypeString},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			toFlip := cmd.Args[0].Str()

			out := ""
			for _, r := range toFlip {
				out = string(r) + out
			}
			bot.Session.ChannelMessageSend(m.ChannelID, "Flippa: "+out)

			return nil
		},
	},
	&commandsystem.SimpleCommand{
		Name:         "Weather",
		Aliases:      []string{"w"},
		Description:  "Shows the weather somewhere",
		RunInDm:      true,
		RequiredArgs: 1,
		Arguments: []*commandsystem.ArgumentDef{
			&commandsystem.ArgumentDef{Name: "Where", Description: "Where", Type: commandsystem.ArgumentTypeString},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
			where := cmd.Args[0].Str()

			req, err := http.NewRequest("GET", "http://wttr.in/"+where, nil)
			if err != nil {
				return err
			}

			req.Header.Set("User-Agent", "curl/7.49.1")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			// remove escape sequences
			unescaped := vtclean.Clean(string(body), false)

			split := strings.Split(string(unescaped), "\n")

			out := "```\n"
			for i := 0; i < 16; i++ {
				if i >= len(split) {
					break
				}
				out += strings.TrimRight(split[i], " ") + "\n"
			}
			out += "\n```"

			_, err = bot.Session.ChannelMessageSend(m.ChannelID, out)
			return err
		},
	},
}
