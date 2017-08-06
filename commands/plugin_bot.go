package commands

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/alfredxing/calc/compute"
	"github.com/jonas747/dice"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/lunixbochs/vtclean"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/tkuchiki/go-timezone"
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

	CommandSystem.DefaultDMHandler = &commandsystem.Command{
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "Unknwon command, only a subset of commands are available in dms.", nil
		},
	}

	CommandSystem.Prefix = p
	CommandSystem.RegisterCommands(GlobalCommands...)
	CommandSystem.RegisterCommands(debugCommands...)

	eventsystem.AddHandler(bot.RedisWrapper(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(HandleMessageCreate, eventsystem.EventMessageCreate)
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

	out += "\n[Debug] # Commands to help debug issues."
	out += generateComandsHelp(categories[CategoryDebug]) + "\n"

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
				if data.Source != commandsystem.SourceDM && target == "" {
					prefix, err := GetCommandPrefix(data.Context().Value(CtxKeyRedisClient).(*redis.Client), data.Guild.ID())
					if err != nil {
						return "Error communicating with redis", err
					}

					footer = "**No command prefix set, you can still use commands through mentioning the bot\n**"
					if prefix != "" {
						footer = fmt.Sprintf("**Command prefix: %q**\n", prefix)
					}
				}

				if target == "" {
					footer += "**Support server:** https://discord.gg/0vYlUK2XBKldPSMY\n**Control Panel:** https://yagpdb.xyz/manage\n"
				}

				channelId := data.Message.ChannelID

				help := GenerateHelp(target)
				if target == "" && data.Source != commandsystem.SourceDM {
					privateChannel, err := bot.GetCreatePrivateChannel(data.Message.Author.ID)
					if err != nil {
						return "", err
					}
					channelId = privateChannel.ID
				}

				if help == "" {
					help = "Command not found"
				}

				dutil.SplitSendMessagePS(common.BotSession, channelId, help+"\n"+footer, "```ini\n", "```", false, false)
				if data.Source != commandsystem.SourceDM && target == "" {
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
		Category: CategoryDebug,
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

				for _, v := range common.Plugins {
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

				return ":upside_down: " + out, nil
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
This bot focuses on being configurable and therefore is one of the more advanced bots.
It can perform a range of general purpose functionality (reddit feeds, various commands, moderation utilities, automoderator functionality and so on) and it's configured through a web control panel.
I'm currently being ran and developed by jonas747#3124 (105487308693757952) but the bot is open source (<https://github.com/jonas747/yagpdb>), so if you know go and want to make some contributions, DM me.
Control panel: <https://yagpdb.xyz/manage>
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

				return fmt.Sprintf("Result: `%f`", result), nil
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
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:        "CatFact",
			Aliases:     []string{"cf", "cat", "catfacts"},
			Description: "Cat Facts",

			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				cf := Catfacts[rand.Intn(len(Catfacts))]
				return cf, nil
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
		Cooldown: 5,
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
	&CustomCommand{
		Category: CategoryFun,
		Command: &commandsystem.Command{
			Name:         "CustomEmbed",
			Aliases:      []string{"ce"},
			Description:  "Creates an embed from what you give it in json form: https://discordapp.com/developers/docs/resources/channel#embed-object",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				{Name: "Json", Type: commandsystem.ArgumentString},
			},
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				var parsed *discordgo.MessageEmbed
				err := json.Unmarshal([]byte(data.SafeArgString(0)), &parsed)
				if err != nil {
					return "Failed parsing json: " + err.Error(), err
				}
				return parsed, nil
			},
		},
	},
	&CustomCommand{
		Category: CategoryTool,
		Command: &commandsystem.Command{
			Name:           "CurrentTime",
			Aliases:        []string{"ctime", "gettime"},
			Description:    "Shows current time in different timezones",
			ArgumentCombos: [][]int{[]int{1}, []int{0}, []int{}},
			Arguments: []*commandsystem.ArgDef{
				{Name: "Zone", Type: commandsystem.ArgumentString},
				{Name: "Offset", Type: commandsystem.ArgumentNumber},
			},
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				const format = "Mon Jan 02 15:04:05 (UTC -07:00)"

				now := time.Now()
				if data.Args[0] != nil {
					tzName := data.Args[0].Str()
					names, err := timezone.GetTimezones(strings.ToUpper(data.Args[0].Str()))
					if err == nil && len(names) > 0 {
						tzName = names[0]
					}

					location, err := time.LoadLocation(tzName)
					if err != nil {
						if offset, ok := customTZOffsets[strings.ToUpper(tzName)]; ok {
							location = time.FixedZone(tzName, int(offset*60*60))
						} else {
							return err, err
						}
					}
					return now.In(location).Format(format), nil
				} else if data.Args[1] != nil {
					location := time.FixedZone("", data.Args[1].Int()*60*60)
					return now.In(location).Format(format), nil
				}

				// No offset of zone specified, just return the bots location
				return now.Format(format), nil
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

func HandleGuildCreate(evt *eventsystem.EventData) {
	client := bot.ContextRedis(evt.Context())
	g := evt.GuildCreate
	prefixExists, err := common.RedisBool(client.Cmd("EXISTS", "command_prefix:"+g.ID))
	if err != nil {
		log.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		client.Cmd("SET", "command_prefix:"+g.ID, "-")
		log.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (-)")
	}
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate
	CommandSystem.HandleMessageCreate(common.BotSession, m)

	bUser := bot.State.User(true)
	if bUser == nil {
		return
	}

	if bUser.ID != m.Author.ID {
		return
	}

	// ping pong
	split := strings.Split(m.Content, ";")
	if split[0] != ":PONG" || len(split) < 2 {
		return
	}

	parsed, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return
	}

	taken := time.Duration(time.Now().UnixNano() - parsed)

	started := time.Now()
	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "Gatway (http send -> gateway receive time): "+taken.String())
	httpPing := time.Since(started)

	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "HTTP API (Edit Msg): "+httpPing.String()+"\nGatway: "+taken.String())
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
