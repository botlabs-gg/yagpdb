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
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"sort"
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

			sysMem, err := mem.VirtualMemory()
			sysMemStats := ""
			if err == nil {
				sysMemStats = fmt.Sprintf("%dMB (%.0f%%), %dMB", sysMem.Used/1000000, sysMem.UsedPercent, sysMem.Total/100000)
			} else {
				sysMemStats = "Failed collecting mem stats"
				log.WithError(err).Error("Failed collecting memory stats")
			}

			sysLoad, err := load.Avg()
			sysLoadStats := ""
			if err == nil {
				sysLoadStats = fmt.Sprintf("%.2f, %.2f, %.2f", sysLoad.Load1, sysLoad.Load5, sysLoad.Load15)
			} else {
				sysLoadStats = "Failed collecting"
				log.WithError(err).Error("Failed collecting load stats")
			}

			uptime := time.Since(bot.Started)

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
					&discordgo.MessageEmbedField{Name: "Servers", Value: fmt.Sprint(servers), Inline: true},
					&discordgo.MessageEmbedField{Name: "Go version", Value: runtime.Version(), Inline: true},
					&discordgo.MessageEmbedField{Name: "Uptime", Value: common.HumanizeDuration(common.DurationPrecisionSeconds, uptime), Inline: true},
					&discordgo.MessageEmbedField{Name: "Goroutines", Value: fmt.Sprint(numGoroutines), Inline: true},
					&discordgo.MessageEmbedField{Name: "GC Pause Fraction", Value: fmt.Sprintf("%.3f%%", memStats.GCCPUFraction*100), Inline: true},
					&discordgo.MessageEmbedField{Name: "Process Mem (alloc, sys, freed)", Value: fmt.Sprintf("%.1fMB, %.1fMB, %.1fMB", float64(memStats.Alloc)/1000000, float64(memStats.Sys)/1000000, (float64(memStats.TotalAlloc)/1000000)-allocated), Inline: true},
					&discordgo.MessageEmbedField{Name: "System Mem (used, total)", Value: sysMemStats, Inline: true},
					&discordgo.MessageEmbedField{Name: "System load (1, 5, 15)", Value: sysLoadStats, Inline: true},
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
			return fmt.Sprintf(":game_die: %d", result+1), nil
		},
	},
	&CustomCommand{
		Cooldown: 10,
		Category: CategoryFun,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "TopServers",
			Description: "Responds with the top 10 servers im on",
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			state := common.BotSession.State
			state.RLock()

			sortable := GuildsSortUsers(state.Guilds)
			sort.Sort(sortable)

			out := "```"
			for k, v := range sortable {
				if k > 10 {
					break
				}

				out += fmt.Sprintf("\n#%-2d: %s (%d)", k, v.Name, v.MemberCount)
			}
			state.RUnlock()
			return out + "\n```", nil
		},
	}, &CustomCommand{
		Cooldown:             10,
		Category:             CategoryFun,
		HideFromCommandsPage: true,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "dumpdm",
			Description:  "dudidadudida",
			HideFromHelp: true,
		},
		RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			if m.Author.ID != common.Conf.Owner {
				return "Only bot owner can run this", nil
			}
			state := common.BotSession.State
			state.RLock()
			defer state.RUnlock()
			out := "Dm channels"
			for _, v := range state.PrivateChannels {
				out += fmt.Sprintf("\n%s - (%s)", v.Recipient.Username, v.ID)
			}
			return out, nil
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
					channelsMem += ChannelMemorySize(channel)

					if channel.Messages != nil {
						totalMessages += len(channel.Messages)
						for _, msg := range channel.Messages {
							messagesMem += GetMessageMemorySize(msg)
						}
					}
				}

				totalMembers += len(v.Members)
				for _, member := range v.Members {
					membersMem += GetMemberMemorySize(member)
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
	if s.State.User == nil || s.State.User.ID != m.Author.ID {
		return
	}

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

// Stuff for measuring memory below
func GetMessageMemorySize(msg *discordgo.Message) int {
	result := int(unsafe.Sizeof(*msg))
	result += len(msg.Content)
	result += len(string(msg.Timestamp))
	if msg.Author != nil {
		result += DiscordUserMemorySize(msg.Author)
	}
	for _, embed := range msg.Embeds {
		result += int(unsafe.Sizeof(*embed))

		if embed.Footer != nil {
			result += len(embed.Footer.IconURL)
			result += len(embed.Footer.ProxyIconURL)
			result += len(embed.Footer.Text)
		}
		if embed.Author != nil {
			result += len(embed.Author.IconURL)
			result += len(embed.Author.Name)
			result += len(embed.Author.ProxyIconURL)
			result += len(embed.Author.URL)
		}
		if embed.Thumbnail != nil {
			result += int(unsafe.Sizeof(*embed.Thumbnail))
			result += len(embed.Thumbnail.ProxyURL)
			result += len(embed.Thumbnail.URL)
		}
		if embed.Provider != nil {
			result += len(embed.Provider.Name)
			result += len(embed.Provider.URL)
		}
		if embed.Video != nil {
			result += int(unsafe.Sizeof(*embed.Video))
			result += len(embed.Video.ProxyURL)
			result += len(embed.Video.URL)
		}
		if embed.Image != nil {
			result += int(unsafe.Sizeof(*embed.Image))
			result += len(embed.Image.ProxyURL)
			result += len(embed.Image.URL)
		}

		for _, field := range embed.Fields {
			result += int(unsafe.Sizeof(*field))
			result += len(field.Name)
			result += len(field.Value)
		}

		result += len(embed.Title)
		result += len(embed.Description)
		result += len(embed.Timestamp)
		result += len(embed.Type)
		result += len(embed.URL)
	}

	return result
}
func ChannelMemorySize(channel *discordgo.Channel) int {
	result := int(unsafe.Sizeof(*channel))
	result += len(channel.Type)
	result += len(channel.Topic)
	result += len(channel.Name)
	result += len(channel.LastMessageID)
	result += len(channel.ID)
	result += len(channel.GuildID)

	if channel.Recipient != nil {
		result += DiscordUserMemorySize(channel.Recipient)
	}

	for _, overwrite := range channel.PermissionOverwrites {
		result += int(unsafe.Sizeof(*overwrite))
		result += len(overwrite.ID)
		result += len(overwrite.Type)
	}

	return result
}

func DiscordUserMemorySize(user *discordgo.User) int {
	result := int(unsafe.Sizeof(*user))
	result += len(user.Avatar)
	result += len(user.Username)
	result += len(user.ID)
	result += len(user.Discriminator)
	result += len(user.Email)
	return result
}

func GetMemberMemorySize(member *discordgo.Member) int {
	result := int(unsafe.Sizeof(*member))
	result += len(member.GuildID)
	result += len(member.JoinedAt)
	result += len(member.Nick)
	if member.User != nil {
		result += DiscordUserMemorySize(member.User)
	}

	for _, role := range member.Roles {
		result += len(role)
	}
	return result
}

type GuildsSortUsers []*discordgo.Guild

func (g GuildsSortUsers) Len() int {
	return len(g)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (g GuildsSortUsers) Less(i, j int) bool {
	return g[i].MemberCount > g[j].MemberCount
}

// Swap swaps the elements with indexes i and j.
func (g GuildsSortUsers) Swap(i, j int) {
	temp := g[i]
	g[i] = g[j]
	g[j] = temp
}
