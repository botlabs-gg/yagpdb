package commands

import (
	"fmt"
	"github.com/alfredxing/calc/compute"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/lunixbochs/vtclean"
	"image"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

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
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Help",
			Description: "Shows help abut all or one specific command",
			RunInDm:     true,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "command", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			target := ""
			if parsed.Args[0] != nil {
				target = parsed.Args[0].Str()
			}

			config := GetConfig(client, parsed.Guild.ID)

			prefixStr := "**No command prefix set, you can still use commands through mentioning the bot\n**"
			if config.Prefix != "" {
				prefixStr = fmt.Sprintf("**Command prefix: %s**\n", config.Prefix)
			}

			help := bot.CommandSystem.GenerateHelp(target, 0)

			privateChannel, err := bot.GetCreatePrivateChannel(common.BotSession, m.Author.ID)
			if err != nil {
				return "", err
			}

			bot.Session.ChannelMessageSend(privateChannel.ID, prefixStr+help)
			return "", nil
		},
	},
	// Status command shows the bot's status, stuff like version, conntected servers, uptime, memory used etc..
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Status",
			Description: "Shows yagpdb status",
			RunInDm:     true,
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			servers := len(bot.Session.State.Guilds)

			uptime := time.Since(bot.Started)

			// Convert to megabytes for ez readings
			allocated := float64(memStats.Alloc) / 1000000
			totalAllocated := float64(memStats.TotalAlloc) / 1000000

			numGoroutines := runtime.NumGoroutine()

			status := fmt.Sprintf("**YAGPDB STATUS** *bot version: %s*\n - Connected Servers: %d\n - Uptime: %s\n - Allocated: %.2fMB\n - Total Allocated: %.2fMB\n - Number of Goroutines: %d\n",
				common.VERSION, servers, uptime.String(), allocated, totalAllocated, numGoroutines)

			return status, nil
		},
	},
	// Some fun commands because why not
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Reverse",
			Aliases:      []string{"r", "rev"},
			Description:  "Flips stuff",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "What", Description: "To flip", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			toFlip := cmd.Args[0].Str()

			out := ""
			for _, r := range toFlip {
				out = string(r) + out
			}
			return out, nil
		},
	},
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Weather",
			Aliases:      []string{"w"},
			Description:  "Shows the weather somewhere",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Where", Description: "Where", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			where := cmd.Args[0].Str()

			req, err := http.NewRequest("GET", "http://wttr.in/"+where, nil)
			if err != nil {
				return err, err
			}

			req.Header.Set("User-Agent", "curl/7.49.1")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err, err
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err, err
			}

			// remove escape sequences
			unescaped := vtclean.Clean(string(body), false)

			split := strings.Split(string(unescaped), "\n")

			out := "```\n"
			for i := 0; i < 7; i++ {
				if i >= len(split) {
					break
				}
				out += strings.TrimRight(split[i], " ") + "\n"
			}
			out += "\n```"

			return out, nil
		},
	},
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Invite",
			Aliases:     []string{"inv", "i"},
			Description: "Responds with bto invite link",
			RunInDm:     true,
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			clientId := bot.Config.ClientID
			link := fmt.Sprintf("https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=535948311&response_type=code&redirect_uri=http://yagpdb.xyz/cp/", clientId)
			return "You manage this bot through the control panel interface but heres an invite link incase you just want that\n" + link, nil
		},
	},
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Ascii",
			Aliases:      []string{"asci"},
			Description:  "Converts an image to ascii",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Where", Description: "Where", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {

			resp, err := http.Get(cmd.Args[0].Str())
			if err != nil {
				return err, err
			}

			img, _, err := image.Decode(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err, err
			}

			out := Convert2Ascii(ScaleImage(img, 50))
			return "```\n" + string(out) + "\n```", nil
		},
	},
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Calc",
			Aliases:      []string{"c", "calculate"},
			Description:  "Calculator 2+2=5",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "What", Description: "What to calculate", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			result, err := compute.Evaluate(cmd.Args[0].Str())
			if err != nil {
				return err, err
			}

			return fmt.Sprintf("Result: `%.20f`", result), nil
		},
	},
	&bot.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Pastebin",
			Aliases:     []string{"ps", "paste"},
			Description: "Creates a pastebin of the channels last 100 messages",
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			id, err := common.CreatePastebinLog(m.ChannelID)
			if err != nil {
				return "Failed uploading to pastebin", err
			}
			return fmt.Sprintf("<http://pastebin.com/%s>", id), nil
		},
	},
}
