package commands

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/alfredxing/calc/compute"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/lunixbochs/vtclean"
	"image"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// calc/compute isnt threadsage :'(
	computeLock   sync.Mutex
	CommandSystem *commandsystem.System
)

func (p *Plugin) InitBot() {

	CommandSystem = commandsystem.NewSystem(common.BotSession, "")
	CommandSystem.SendError = false
	CommandSystem.CensorError = CensorError

	CommandSystem.Prefix = p
	CommandSystem.RegisterCommands(GlobalCommands...)

	common.BotSession.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
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
	prefix, err := GetCommandPrefix(client, channel.GuildID)
	if err != nil {
		log.Println("Failed retrieving prefix", err)
	}
	return prefix
}

var GlobalCommands = []commandsystem.CommandHandler{
	&CustomCommand{
		Cooldown: 10,
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

			// Fetch the prefix if ther command was not run in a dm
			prefixStr := ""
			if parsed.Source != commandsystem.CommandSourceDM {
				prefix, err := GetCommandPrefix(client, parsed.Guild.ID)
				if err != nil {
					return "Error communicating with redis", err
				}

				prefixStr = "**No command prefix set, you can still use commands through mentioning the bot\n**"
				if prefix != "" {
					prefixStr = fmt.Sprintf("**Command prefix: %q**\n", prefix)
				}
			}

			help := CommandSystem.GenerateHelp(target, 0)

			privateChannel, err := bot.GetCreatePrivateChannel(common.BotSession, m.Author.ID)
			if err != nil {
				return "", err
			}

			bot.Session.ChannelMessageSend(privateChannel.ID, prefixStr+help)
			return "", nil
		},
	},
	// Status command shows the bot's status, stuff like version, conntected servers, uptime, memory used etc..
	&CustomCommand{
		Cooldown: 2,
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
	&CustomCommand{
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
	&CustomCommand{
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
	&CustomCommand{
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
	&CustomCommand{
		Cooldown: 20,
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
	&CustomCommand{
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
			computeLock.Lock()
			defer computeLock.Unlock()
			result, err := compute.Evaluate(cmd.Args[0].Str())
			if err != nil {
				return err, err
			}

			return fmt.Sprintf("Result: `%G`", result), nil
		},
	},
	&CustomCommand{
		Cooldown: 30,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Hastebin",
			Aliases:     []string{"ps", "paste", "pastebin"},
			Description: "Creates a hastebin of the channels last 100 messages",
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			id, err := common.CreateHastebinLog(m.ChannelID)
			if err != nil {
				return "Failed uploading to hastebin", err
			}
			return fmt.Sprintf("Link: <%s>", id), nil
		},
	},
	&CustomCommand{
		Cooldown: 5,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Topic",
			Description: "Generates a chat topic",
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			doc, err := goquery.NewDocument("http://www.conversationstarters.com/generator.php")
			if err != nil {
				return err, err
			}

			topic := doc.Find("#random").Text()
			return topic, nil
		},
	},
	&CustomCommand{
		Cooldown: 5,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "CatFact",
			Aliases:     []string{"cf", "cat", "catfacts"},
			Description: "Cat Facts",
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			resp, err := http.Get("http://catfacts-api.appspot.com/api/facts")
			if err != nil {
				return err, err
			}

			decoded := struct {
				Facts   []string `json:"facts"`
				Success string   `json:"success"`
			}{}

			err = json.NewDecoder(resp.Body).Decode(&decoded)
			if err != nil {
				return err, err
			}

			fact := "No catfact :'("

			if decoded.Success == "true" && len(decoded.Facts) > 0 {
				fact = decoded.Facts[0]
			}

			return fact, nil
		},
	},
	&CustomCommand{
		Cooldown: 5,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Advice",
			Description: "Get a advice",
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "What", Description: "What to get advice on", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			random := true
			addr := "http://api.adviceslip.com/advice"
			if cmd.Args[0] != nil {
				random = false
				addr = "http://api.adviceslip.com/advice/search/" + url.QueryEscape(cmd.Args[0].Str())
			}

			resp, err := http.Get(addr)
			if err != nil {
				return err, err
			}

			var decoded interface{}

			if random {
				decoded = &RandomAdviceResp{}
			} else {
				decoded = &SearchAdviceResp{}
			}

			err = json.NewDecoder(resp.Body).Decode(&decoded)
			if err != nil {
				return err, err
			}

			advice := "No advice found :'("

			if random {
				slip := decoded.(*RandomAdviceResp).Slip
				if slip != nil {
					advice = slip.Advice
				}
			} else {
				cast := decoded.(*SearchAdviceResp)
				if len(cast.Slips) > 0 {
					advice = cast.Slips[0].Advice
				}
			}

			return advice, nil
		},
	},
}

type AdviceSlip struct {
	Advice string `json:"advice"`
	ID     string `json:"slip_id"`
}

type RandomAdviceResp struct {
	Slip *AdviceSlip `json:"slip"`
}

type SearchAdviceResp struct {
	TotalResults json.Number   `json:"total_results"`
	Slips        []*AdviceSlip `json:"slips"`
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	prefixExists, err := client.Cmd("EXISTS", "command_prefix:"+g.ID).Bool()
	if err != nil {
		log.Println("Failed checkign if prefix exists")
		return
	}

	if !prefixExists {
		client.Cmd("SET", "command_prefix:"+g.ID, "-")
		log.Println("Set command prefix to default of -")
	}
}
