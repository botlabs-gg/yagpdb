package stdcommands

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/alfredxing/calc/compute"
	"github.com/jonas747/dice"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/lunixbochs/vtclean"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/tkuchiki/go-timezone"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// calc/compute isnt threadsafe :'(
	computeLock sync.Mutex
)

type PluginStatus interface {
	Status(client *redis.Client) (string, string)
}

var generalCommands = []commandsystem.CommandHandler{
	cmdReverse,
	cmdWeather,
	cmdCalc,
	cmdTopic,
	cmdCatFact,
	cmdAdvice,
	cmdPing,
	cmdThrow,
	cmdRoll,
	cmdCustomEmbed,
	cmdCurrentTime,
	cmdMentionRole,
}

var cmdReverse = &commands.CustomCommand{
	Category: commands.CategoryFun,
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
}

var cmdWeather = &commands.CustomCommand{
	Category: commands.CategoryFun,
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
}

var cmdCalc = &commands.CustomCommand{
	Category: commands.CategoryTool,
	Command: &commandsystem.Command{
		Name:         "Calc",
		Aliases:      []string{"c", "calculate"},
		Description:  "Calculator 2+2=5",
		RunInDm:      true,
		RequiredArgs: 1,
		Arguments: []*commandsystem.ArgDef{
			&commandsystem.ArgDef{Name: "Expression", Description: "What to calculate", Type: commandsystem.ArgumentString},
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
}

var cmdTopic = &commands.CustomCommand{
	Cooldown: 5,
	Category: commands.CategoryFun,
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
}

var cmdCatFact = &commands.CustomCommand{
	Category: commands.CategoryFun,
	Command: &commandsystem.Command{
		Name:        "CatFact",
		Aliases:     []string{"cf", "cat", "catfacts"},
		Description: "Cat Facts",

		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			cf := Catfacts[rand.Intn(len(Catfacts))]
			return cf, nil
		},
	},
}

var cmdAdvice = &commands.CustomCommand{
	Cooldown: 5,
	Category: commands.CategoryFun,
	Command: &commandsystem.Command{
		Name:        "Advice",
		Description: "Get advice",
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
}

var cmdPing = &commands.CustomCommand{
	Category: commands.CategoryTool,
	Command: &commandsystem.Command{
		Name:        "Ping",
		Description: "I prefer tabletennis",

		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return fmt.Sprintf(":PONG;%d", time.Now().UnixNano()), nil
		},
	},
}

var cmdThrow = &commands.CustomCommand{
	Category: commands.CategoryFun,
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
}

var cmdRoll = &commands.CustomCommand{
	Category: commands.CategoryFun,
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
}

var cmdCustomEmbed = &commands.CustomCommand{
	Category: commands.CategoryFun,
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
}

var cmdCurrentTime = &commands.CustomCommand{
	Category: commands.CategoryTool,
	Command: &commandsystem.Command{
		Name:           "CurrentTime",
		Aliases:        []string{"ctime", "gettime"},
		Description:    "Shows current time in different timezones",
		ArgumentCombos: [][]int{[]int{1}, []int{0}, []int{}},
		Arguments: []*commandsystem.ArgDef{
			{Name: "Zone", Type: commandsystem.ArgumentString},
			{Name: "Offset", Type: commandsystem.ArgumentNumber},
		},
		Run: cmdFuncCurrentTime,
	},
}

func cmdFuncCurrentTime(data *commandsystem.ExecData) (interface{}, error) {
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
}

var cmdMentionRole = &commands.CustomCommand{
	Category: commands.CategoryTool,
	Command: &commandsystem.Command{
		Name:            "MentionRole",
		Aliases:         []string{"mrole"},
		Description:     "Sets a role to mentionable, mentions the role, and then sets it back",
		LongDescription: "Requires the manage roles permission and the bot being above the mentioned role",
		RequiredArgs:    1,
		Arguments: []*commandsystem.ArgDef{
			{Name: "Role", Type: commandsystem.ArgumentString},
		},
		Run: cmdFuncMentionRole,
	},
}

func cmdFuncMentionRole(data *commandsystem.ExecData) (interface{}, error) {
	if ok, err := bot.AdminOrPerm(discordgo.PermissionManageServer, data.Message.Author.ID, data.Channel.ID()); err != nil {
		return "Failed checking perms", err
	} else if !ok {
		return "You need manage server perms to use this commmand", nil
	}

	var role *discordgo.Role
	data.Guild.RLock()
	defer data.Guild.RUnlock()
	for _, r := range data.Guild.Guild.Roles {
		if strings.EqualFold(r.Name, data.Args[0].Str()) {
			role = r
			break
		}
	}

	if role == nil {
		return "No role with the name `" + data.Args[0].Str() + "` found", nil
	}

	_, err := common.BotSession.GuildRoleEdit(data.Guild.ID(), role.ID, role.Name, role.Color, role.Hoist, role.Permissions, true)
	if err != nil {
		if _, dErr := common.DiscordError(err); dErr != "" {
			return "Failed updating role, discord responded with: " + dErr, err
		} else {
			return "An unknown error occured updating the role", err
		}
	}

	_, err = common.BotSession.ChannelMessageSend(data.Channel.ID(), "<@&"+role.ID+">")

	common.BotSession.GuildRoleEdit(data.Guild.ID(), role.ID, role.Name, role.Color, role.Hoist, role.Permissions, false)
	return "", err
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

func HandleMessageCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate

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
