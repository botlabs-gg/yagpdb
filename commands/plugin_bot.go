package commands

import (
	"context"
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
	"github.com/justinian/dice"
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

	CommandSystem = commandsystem.NewSystem(nil, "")
	CommandSystem.SendError = false
	CommandSystem.CensorError = CensorError
	CommandSystem.State = bot.State

	CommandSystem.Prefix = p
	CommandSystem.RegisterCommands(GlobalCommands...)
	CommandSystem.RegisterCommands(debugCommands...)

	bot.AddHandler(bot.RedisWrapper(HandleGuildCreate), bot.EventGuildCreate)
	bot.AddHandler(HandleMessageCreate, bot.EventMessageCreate)
}

func (p *Plugin) GetPrefix(s *discordgo.Session, m *discordgo.MessageCreate) string {
	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection from pool")
		return ""
	}
	defer common.RedisPool.Put(client)

	channel := bot.State.Channel(true, m.ChannelID)
	if channel == nil {
		log.Error("Failed retrieving channels from state")
		return ""
	}

	prefix, err := GetCommandPrefix(client, channel.Guild.ID())
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
		Command: &commandsystem.Command{
			Name:        "Help",
			Description: "Shows help abut all or one specific command",
			RunInDm:     true,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "command", Type: commandsystem.ArgumentString},
			},

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				target := ""
				if data.Args[0] != nil {
					target = data.Args[0].Str()
				}

				// Fetch the prefix if ther command was not run in a dm
				footer := ""
				if data.Source != commandsystem.SourceDM {
					prefix, err := GetCommandPrefix(data.Context().Value(CtxKeyRedisClient).(*redis.Client), data.Guild.ID())
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

				privateChannel, err := bot.GetCreatePrivateChannel(data.Message.Author.ID)
				if err != nil {
					return "", err
				}

				dutil.SplitSendMessagePS(common.BotSession, privateChannel.ID, help+"\n"+footer, "```ini\n", "```", false, false)
				if data.Source != commandsystem.SourceDM {
					return "You've Got Mail!", nil
				} else {
					return "", nil
				}
			},
		},
	},
	// Status command shows the bot's status, stuff like version, conntected servers, uptime, memory used etc..
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Yagstatus",
			Aliases:     []string{"Status"},
			Description: "Shows yagpdb status",
			RunInDm:     true,

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				bot.State.RLock()
				servers := len(bot.State.Guilds)
				bot.State.RUnlock()

				sysMem, err := mem.VirtualMemory()
				sysMemStats := ""
				if err == nil {
					sysMemStats = fmt.Sprintf("%dMB (%.0f%%), %dMB", sysMem.Used/1000000, sysMem.UsedPercent, sysMem.Total/1000000)
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

				numScheduledEvent, _ := common.NumScheduledEvents(data.Context().Value(CtxKeyRedisClient).(*redis.Client))

				botUser := bot.State.User(true)

				embed := &discordgo.MessageEmbed{
					Author: &discordgo.MessageEmbedAuthor{
						Name:    botUser.Username,
						IconURL: discordgo.EndpointUserAvatar(botUser.ID, botUser.Avatar),
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
						name, val := cast.Status(data.Context().Value(CtxKeyRedisClient).(*redis.Client))
						if name == "" || val == "" {
							continue
						}
						embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: v.Name() + ": " + name, Value: val, Inline: true})
					}
				}

				return &commandsystem.FallbackEmebd{embed}, nil
			},
		},
	},
	// Some fun commands because why not
	&CustomCommand{
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:         "Reverse",
			Aliases:      []string{"r", "rev"},
			Description:  "Reverses the text given",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "What", Description: "To flip", Type: commandsystem.ArgumentString},
			},

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				toFlip := data.Args[0].Str()

				out := ""
				for _, r := range toFlip {
					out = string(r) + out
				}
				return out, nil
			},
		},
	},
	&CustomCommand{
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:         "Weather",
			Aliases:      []string{"w"},
			Description:  "Shows the weather somewhere (add ?m for metric: -w bergen?m)",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "Where", Description: "Where", Type: commandsystem.ArgumentString},
			},
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				where := data.Args[0].Str()

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
	},
	&CustomCommand{
		Category: CategoryGeneral,
		Command: &commandsystem.Command{
			Name:        "Invite",
			Aliases:     []string{"inv", "i"},
			Description: "Responds with bot invite link",
			RunInDm:     true,

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				return "Please add the bot through the websie\nhttps://" + common.Conf.Host, nil
			},
		},
	},
	&CustomCommand{
		Category: CategoryGeneral,
		Command: &commandsystem.Command{
			Name:        "Info",
			Description: "Responds with bot information",
			RunInDm:     true,
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				const info = `**YAGPDB - Yet Another General Purpose Discord Bot**
This bot focuses on being configurable and therefor is one of the more advanced bots.
I can perform a range of general purpose functionality (reddit feeds, various commands, moderation utilities, automoderator functionality and so on) and im configured through a web control panel.
I'm currently being ran and developed by jonas747#3124 but the bot is open source (<https://github.com/jonas747/yagpdb>), so if you know go and want to make some contributions, DM me.
				`
				return info, nil
			},
		},
	},
	&CustomCommand{
		Category: CategoryTool,
		Command: &commandsystem.Command{
			Name:         "Calc",
			Aliases:      []string{"c", "calculate"},
			Description:  "Calculator 2+2=5",
			RunInDm:      true,
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "What", Description: "What to calculate", Type: commandsystem.ArgumentString},
			},

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				computeLock.Lock()
				defer computeLock.Unlock()
				result, err := compute.Evaluate(data.Args[0].Str())
				if err != nil {
					return err, err
				}

				return fmt.Sprintf("Result: `%G`", result), nil
			},
		},
	},
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Topic",
			Description: "Generates a chat topic",

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				doc, err := goquery.NewDocument("http://www.conversationstarters.com/generator.php")
				if err != nil {
					return err, err
				}

				topic := doc.Find("#random").Text()
				return topic, nil
			},
		},
	},
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "CatFact",
			Aliases:     []string{"cf", "cat", "catfacts"},
			Description: "Cat Facts",

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
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
	},
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Advice",
			Description: "Get a advice",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "What", Description: "What to get advice on", Type: commandsystem.ArgumentString},
			},

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				random := true
				addr := "http://api.adviceslip.com/advice"
				if data.Args[0] != nil {
					random = false
					addr = "http://api.adviceslip.com/advice/search/" + url.QueryEscape(data.Args[0].Str())
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
	},
	&CustomCommand{
		Cooldown: 5,
		Category: CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Ping",
			Description: "I prefer tabletennis",

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				return fmt.Sprintf(":PONG;%d", time.Now().UnixNano()), nil
			},
		},
	},
	&CustomCommand{
		Cooldown: 2,
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Throw",
			Description: "Cause you are a rebel",
			Arguments: []*commandsystem.ArgDef{
				{Name: "Target", Type: commandsystem.ArgumentUser},
			},

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				thing := common.Things[rand.Intn(len(common.Things))]

				target := "a random person nearby"
				if data.Args[0] != nil {
					target = data.Args[0].DiscordUser().Username
				}

				return fmt.Sprintf("Threw **%s** at %s", thing, target), nil
			},
		},
	},
	&CustomCommand{
		Cooldown: 2,
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Roll",
			Description: "Roll dices, specify nothing for 6 sides, specify a number for max sides, or rpg dice syntax",
			Arguments: []*commandsystem.ArgDef{
				{Name: "Dice Desc", Type: commandsystem.ArgumentString},
				{Name: "Sides", Type: commandsystem.ArgumentNumber},
			},
			ArgumentCombos: [][]int{[]int{1}, []int{0}, []int{}},
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				if data.Args[0] != nil {
					// Special dice syntax if string
					r, _, err := dice.Roll(data.Args[0].Str())
					if err != nil {
						return err.Error(), nil
					}
					return r.String(), nil
				}

				// normal, n sides dice rolling
				sides := data.SafeArgInt(1)
				if sides < 1 {
					sides = 6
				}

				result := rand.Intn(sides)
				return fmt.Sprintf(":game_die: %d (1 - %d)", result+1, sides), nil
			},
		},
	},
	&CustomCommand{
		Cooldown: 10,
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "TopServers",
			Description: "Responds with the top 10 servers im on",

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				state := bot.State
				state.RLock()

				guilds := make([]*discordgo.Guild, len(state.Guilds))
				i := 0
				for _, v := range state.Guilds {
					state.RUnlock()
					guilds[i] = v.LightCopy(true)
					state.RLock()
					i++
				}
				state.RUnlock()

				sortable := GuildsSortUsers(guilds)
				sort.Sort(sortable)

				out := "```"
				for k, v := range sortable {
					if k > 9 {
						break
					}

					out += fmt.Sprintf("\n#%-2d: %-25s (%d members)", k+1, v.Name, v.MemberCount)
				}
				return out + "\n```", nil
			},
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

func HandleGuildCreate(ctx context.Context, evt interface{}) {
	client := bot.ContextRedis(ctx)
	g := evt.(*discordgo.GuildCreate)
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

func HandleMessageCreate(ctx context.Context, evt interface{}) {
	m := evt.(*discordgo.MessageCreate)
	CommandSystem.HandleMessageCreate(bot.ContextSession(ctx), m)

	if bot.State.User(true).ID != m.Author.ID {
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
	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "Received pong, took: "+taken.String())
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

type HIBPBread struct {
	Name        string
	Title       string
	BreachDate  string
	Description string
	Domain      string
	AddedDate   string
	DataClasses []string
	PwnCount    int
	IsVerified  bool
	IsSensitive bool
	IsSpamList  bool
	IsRetired   bool
}
