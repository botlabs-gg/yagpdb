package commands

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/alfredxing/calc/compute"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/lunixbochs/vtclean"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	// calc/compute isnt threadsafe :'(
	computeLock   sync.Mutex
	CommandSystem *commandsystem.System
)

type PluginStatus interface {
	Status(client *redis.Client) (string, string)
}

func (p *Plugin) InitBot() {

	CommandSystem = commandsystem.NewSystem(common.BotSession, "")
	CommandSystem.SendError = false
	CommandSystem.CensorError = CensorError

	CommandSystem.Prefix = p
	CommandSystem.RegisterCommands(GlobalCommands...)

	common.BotSession.AddHandler(bot.CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(bot.CustomMessageCreate(HandleMessageCreate))
}

func (p *Plugin) GetPrefix(s *discordgo.Session, m *discordgo.MessageCreate) string {
	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection from pool")
		return ""
	}
	defer common.RedisPool.Put(client)

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving channels from state")
		return ""
	}
	prefix, err := GetCommandPrefix(client, channel.GuildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving commands prefix")
	}
	return prefix
}

func GenerateHelp(target string) string {
	if target != "" {
		return CommandSystem.GenerateHelp(target, 100)
	}

	categories := make(map[CommandCategory][]*CustomCommand)

	for _, v := range CommandSystem.Commands {
		cast := v.(*CustomCommand)
		categories[cast.Category] = append(categories[cast.Category], cast)
	}

	out := "```ini\n"

	out += `[Legend]
# 
#Command   = {alias1, alias2...} <required arg> (optional arg) : Description
#
#Example:
Help        = {hlp}   (command)       : blablabla
# |             |          |                |
#Comand name, Aliases,  optional arg,    Description

`

	// Do it manually to preserve order
	out += "[General] # General YAGPDB commands"
	out += generateComandsHelp(categories[CategoryGeneral]) + "\n"

	out += "\n[Tools]"
	out += generateComandsHelp(categories[CategoryTool]) + "\n"

	out += "\n[Moderation] # These are off by default"
	out += generateComandsHelp(categories[CategoryModeration]) + "\n"

	out += "\n[Misc/Fun] # Fun commands for family and friends!"
	out += generateComandsHelp(categories[CategoryFun]) + "\n"

	unknown, ok := categories[CommandCategory("")]
	if ok && len(unknown) > 1 {
		out += "\n[Unknown] # ??"
		out += generateComandsHelp(unknown) + "\n"
	}

	out += "```"
	return out
}

func generateComandsHelp(cmds []*CustomCommand) string {
	out := ""
	for _, v := range cmds {
		if !v.HideFromHelp {
			out += "\n" + v.GenerateHelp("", 100, 0)
		}
	}
	return out
}

var GlobalCommands = []commandsystem.CommandHandler{
	&CustomCommand{
		Cooldown: 10,
		Category: CategoryGeneral,
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
			footer := ""
			if parsed.Source != commandsystem.CommandSourceDM {
				prefix, err := GetCommandPrefix(client, parsed.Guild.ID)
				if err != nil {
					return "Error communicating with redis", err
				}

				footer = "**No command prefix set, you can still use commands through mentioning the bot\n**"
				if prefix != "" {
					footer = fmt.Sprintf("**Command prefix: %q**\n", prefix)
				}
			}
			footer += "**Support server:** https://discord.gg/0vYlUK2XBKldPSMY\n"

			help := GenerateHelp(target)

			privateChannel, err := bot.GetCreatePrivateChannel(common.BotSession, m.Author.ID)
			if err != nil {
				return "", err
			}

			dutil.SplitSendMessagePS(common.BotSession, privateChannel.ID, help+"\n"+footer, "```ini\n", "```", false, false)
			//dutil.SplitSendMessage(common.BotSession, privateChannel.ID, prefixStr+help)
			return "You've Got Mail!", nil
		},
	},
	// Status command shows the bot's status, stuff like version, conntected servers, uptime, memory used etc..
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Yagstatus",
			Aliases:     []string{"Status"},
			Description: "Shows yagpdb status",
			RunInDm:     true,
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			servers := len(common.BotSession.State.Guilds)

			uptime := time.Since(bot.Started)

			// Convert to megabytes for ez readings
			allocated := float64(memStats.Alloc) / 1000000

			numGoroutines := runtime.NumGoroutine()

			numScheduledEvent, _ := common.NumScheduledEvents(client)

			embed := &discordgo.MessageEmbed{
				Author: &discordgo.MessageEmbedAuthor{
					Name:    common.BotSession.State.User.Username,
					IconURL: discordgo.EndpointUserAvatar(common.BotSession.State.User.ID, common.BotSession.State.User.Avatar),
				},
				Title: "YAGPDB Status, version " + common.VERSION,
				Fields: []*discordgo.MessageEmbedField{
					&discordgo.MessageEmbedField{Name: "Number of servers", Value: fmt.Sprint(servers), Inline: true},
					&discordgo.MessageEmbedField{Name: "Uptime", Value: common.HumanizeDuration(common.DurationPrecisionSeconds, uptime), Inline: true},
					&discordgo.MessageEmbedField{Name: "Memory Allocated", Value: fmt.Sprintf("%.1fMB", allocated), Inline: true},
					&discordgo.MessageEmbedField{Name: "Goroutines", Value: fmt.Sprint(numGoroutines), Inline: true},
					&discordgo.MessageEmbedField{Name: "Scheduled events (reminders etc)", Value: fmt.Sprint(numScheduledEvent), Inline: true},
				},
			}

			for _, v := range common.AllPlugins {
				if cast, ok := v.(PluginStatus); ok {
					name, val := cast.Status(client)
					if name == "" || val == "" {
						continue
					}
					embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: v.Name() + ": " + name, Value: val, Inline: true})
				}
			}

			return embed, nil
		},
	},
	// Some fun commands because why not
	&CustomCommand{
		Category: CategoryFun,
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
		Category: CategoryFun,
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
		Category: CategoryGeneral,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Invite",
			Aliases:     []string{"inv", "i"},
			Description: "Responds with bot invite link",
			RunInDm:     true,
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			return "Please add the bot through the websie\nhttps://" + common.Conf.Host, nil
		},
	},
	&CustomCommand{
		Category: CategoryTool,
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
		Cooldown: 5,
		Category: CategoryFun,
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
		Category: CategoryFun,
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
		Category: CategoryFun,
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
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "ping",
			Description: "I prefer tabletennis",
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			return fmt.Sprintf(":PONG;%d", time.Now().UnixNano()), nil
		},
	},
	&CustomCommand{
		Cooldown: 2,
		Category: CategoryFun,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Throw",
			Description: "Cause you are a rebel",
			Arguments: []*commandsystem.ArgumentDef{
				{Name: "Target", Type: commandsystem.ArgumentTypeUser},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			thing := common.Things[rand.Intn(len(common.Things))]

			target := "a random person nearby"
			if cmd.Args[0] != nil {
				target = cmd.Args[0].DiscordUser().Username
			}

			return fmt.Sprintf("Threw **%s** at %s", thing, target), nil
		},
	},
	&CustomCommand{
		Cooldown: 2,
		Category: CategoryFun,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Roll",
			Description: "Roll a dice",
			Arguments: []*commandsystem.ArgumentDef{
				{Name: "Sides", Type: commandsystem.ArgumentTypeNumber},
			},
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			sides := 6
			if cmd.Args[0] != nil && cmd.Args[0].Int() > 0 {
				sides = cmd.Args[0].Int()
			}

			result := rand.Intn(sides)
			return fmt.Sprintf(":game_die: %d", result), nil
		},
	},
	&CustomCommand{
		Cooldown:             2,
		Category:             CategoryFun,
		HideFromCommandsPage: true,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Yagmem",
			Description:  "Send some memory stats",
			HideFromHelp: true,
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			if m.Author.ID != common.Conf.Owner {
				return "Only bot owner can run this", nil
			}

			totalGuilds := 0
			totalMembers := 0
			membersMem := 0
			totalChannels := 0
			channelsMem := 0
			totalMessages := 0
			messagesMem := 0

			state := common.BotSession.State
			state.RLock()
			defer state.RUnlock()

			for _, v := range state.Guilds {
				totalGuilds++

				for _, channel := range v.Channels {
					totalChannels++
					channelsMem += int(unsafe.Sizeof(channel))

					if channel.Messages != nil {
						totalMessages += len(channel.Messages)
						for _, msg := range channel.Messages {
							messagesMem += int(unsafe.Sizeof(msg))
						}
					}
				}

				totalMembers += len(v.Members)
				for _, member := range v.Members {
					membersMem += int(unsafe.Sizeof(member))
				}
			}

			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)

			embed := &discordgo.MessageEmbed{
				Title: "Memory stats",
				Fields: []*discordgo.MessageEmbedField{
					&discordgo.MessageEmbedField{Name: "Allocated", Value: fmt.Sprintf("%dMB", stats.Alloc/1000000), Inline: true},
					&discordgo.MessageEmbedField{Name: "TotalAlloc", Value: fmt.Sprintf("%dMB", stats.TotalAlloc/1000000), Inline: true},
					&discordgo.MessageEmbedField{Name: "Guilds", Value: fmt.Sprint(totalGuilds), Inline: true},
					&discordgo.MessageEmbedField{Name: "Members", Value: fmt.Sprintf("%d (%.1fKB)", totalMembers, float64(membersMem)/1000), Inline: true},
					&discordgo.MessageEmbedField{Name: "Messages", Value: fmt.Sprintf("%d (%.1fKB)", totalMessages, float64(messagesMem)/1000), Inline: true},
					&discordgo.MessageEmbedField{Name: "Channels", Value: fmt.Sprintf("%d (%.1fKB)", totalChannels, float64(channelsMem)/1000), Inline: true},
				},
			}

			return embed, nil
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
		log.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		client.Cmd("SET", "command_prefix:"+g.ID, "-")
		log.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (-)")
	}
}

func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate, client *redis.Client) {
	split := strings.Split(m.Content, ";")
	if split[0] != ":PONG" || len(split) < 2 {
		return
	}

	parsed, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return
	}

	taken := time.Duration(time.Now().UnixNano() - parsed)
	s.ChannelMessageEdit(m.ChannelID, m.ID, "Received pong, took: "+taken.String())
}
