package stdcommands

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/alfredxing/calc/compute"
	"github.com/dpatrie/urbandictionary"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dice"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/lunixbochs/vtclean"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"github.com/tkuchiki/go-timezone"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
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

var generalCommands = []*commands.YAGCommand{
	cmdDefine,
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
	cmdListRoles,
	cmdWouldYouRather,
}

var cmdReverse = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Reverse",
	Aliases:      []string{"r", "rev"},
	Description:  "Reverses the text given",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "What", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		toFlip := data.Args[0].Str()

		out := ""
		for _, r := range toFlip {
			out = string(r) + out
		}

		return ":upside_down: " + out, nil
	},
}

var cmdWeather = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Weather",
	Aliases:      []string{"w"},
	Description:  "Shows the weather somewhere (add ?m for metric: -w bergen?m)",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Where", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
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
}

var cmdCalc = &commands.YAGCommand{
	CmdCategory:  commands.CategoryTool,
	Name:         "Calc",
	Aliases:      []string{"c", "calculate"},
	Description:  "Calculator 2+2=5",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Expression", Type: dcmd.String},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		computeLock.Lock()
		defer computeLock.Unlock()
		result, err := compute.Evaluate(data.Args[0].Str())
		if err != nil {
			return err, err
		}

		return fmt.Sprintf("Result: `%f`", result), nil
	},
}

var cmdTopic = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Topic",
	Description: "Generates a chat topic",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		doc, err := goquery.NewDocument("http://www.conversationstarters.com/generator.php")
		if err != nil {
			return err, err
		}

		topic := doc.Find("#random").Text()
		return topic, nil
	},
}

var cmdCatFact = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "CatFact",
	Aliases:     []string{"cf", "cat", "catfacts"},
	Description: "Cat Facts",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		cf := Catfacts[rand.Intn(len(Catfacts))]
		return cf, nil
	},
}

var cmdAdvice = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Advice",
	Description: "Get advice",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "What", Type: dcmd.String},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		random := true
		addr := "http://api.adviceslip.com/advice"
		if data.Args[0].Str() != "" {
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
}

var cmdPing = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Ping",
	Description: "I prefer tabletennis (Shows the bots ping to the discord servers)",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		return fmt.Sprintf(":PONG;%d", time.Now().UnixNano()), nil
	},
}

var cmdThrow = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Throw",
	Description: "Cause you are a rebel",
	Arguments: []*dcmd.ArgDef{
		{Name: "Target", Type: dcmd.User},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		target := "a random person nearby"
		if data.Args[0].Value != nil {
			target = data.Args[0].Value.(*discordgo.User).Username
		}

		resp := ""

		rng := rand.Intn(100)
		if rng < 5 {
			resp = fmt.Sprintf("TRIPPLE THROW! Threw **%s**, **%s** and **%s** at **%s**", RandomThing(), RandomThing(), RandomThing(), target)
		} else if rng < 15 {
			resp = fmt.Sprintf("DOUBLE THROW! Threw **%s** and **%s** at **%s**", RandomThing(), RandomThing(), target)
		} else {
			resp = fmt.Sprintf("Threw **%s** at **%s**", RandomThing(), target)
		}

		return resp, nil
	},
}

var cmdRoll = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Roll",
	Description: "Roll dices, specify nothing for 6 sides, specify a number for max sides, or rpg dice syntax",
	Arguments: []*dcmd.ArgDef{
		{Name: "RPG Dice", Type: dcmd.String},
		{Name: "Sides", Default: 0, Type: dcmd.Int},
	},
	ArgumentCombos: [][]int{[]int{1}, []int{0}, []int{}},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		if data.Args[0].Value != nil {
			// Special dice syntax if string
			r, _, err := dice.Roll(data.Args[0].Str())
			if err != nil {
				return err.Error(), nil
			}
			return r.String(), nil
		}

		// normal, n sides dice rolling
		sides := data.Args[1].Int()
		if sides < 1 {
			sides = 6
		}

		result := rand.Intn(sides)
		return fmt.Sprintf(":game_die: %d (1 - %d)", result+1, sides), nil
	},
}

var cmdCustomEmbed = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "CustomEmbed",
	Aliases:      []string{"ce"},
	Description:  "Creates an embed from what you give it in json form: https://discordapp.com/developers/docs/resources/channel#embed-object",
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Json", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var parsed *discordgo.MessageEmbed
		err := json.Unmarshal([]byte(data.Args[0].Str()), &parsed)
		if err != nil {
			return "Failed parsing json: " + err.Error(), err
		}
		return parsed, nil
	},
}

var cmdCurrentTime = &commands.YAGCommand{
	CmdCategory:    commands.CategoryTool,
	Name:           "CurrentTime",
	Aliases:        []string{"ctime", "gettime"},
	Description:    "Shows current time in different timezones",
	ArgumentCombos: [][]int{[]int{1}, []int{0}, []int{}},
	Arguments: []*dcmd.ArgDef{
		{Name: "Zone", Type: dcmd.String},
		{Name: "Offset", Type: dcmd.Int},
	},
	RunFunc: cmdFuncCurrentTime,
}

func cmdFuncCurrentTime(data *dcmd.Data) (interface{}, error) {
	const format = "Mon Jan 02 15:04:05 (UTC -07:00)"

	now := time.Now()
	if data.Args[0].Value != nil {
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
				return "Unknown timezone :(", err
			}
		}
		return now.In(location).Format(format), nil
	} else if data.Args[1].Value != nil {
		location := time.FixedZone("", data.Args[1].Int()*60*60)
		return now.In(location).Format(format), nil
	}

	// No offset of zone specified, just return the bots location
	return now.Format(format), nil
}

var cmdMentionRole = &commands.YAGCommand{
	CmdCategory:     commands.CategoryTool,
	Name:            "MentionRole",
	Aliases:         []string{"mrole"},
	Description:     "Sets a role to mentionable, mentions the role, and then sets it back",
	LongDescription: "Requires the manage roles permission and the bot being above the mentioned role",
	RequiredArgs:    1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Role", Type: dcmd.String},
	},
	RunFunc: cmdFuncMentionRole,
}

func cmdFuncMentionRole(data *dcmd.Data) (interface{}, error) {
	if ok, err := bot.AdminOrPerm(discordgo.PermissionManageRoles, data.Msg.Author.ID, data.CS.ID()); err != nil {
		return "Failed checking perms", err
	} else if !ok {
		return "You need manage server perms to use this commmand", nil
	}

	var role *discordgo.Role
	data.GS.RLock()
	defer data.GS.RUnlock()
	for _, r := range data.GS.Guild.Roles {
		if strings.EqualFold(r.Name, data.Args[0].Str()) {
			role = r
			break
		}
	}

	if role == nil {
		return "No role with the name `" + data.Args[0].Str() + "` found", nil
	}

	_, err := common.BotSession.GuildRoleEdit(data.GS.ID(), role.ID, role.Name, role.Color, role.Hoist, role.Permissions, true)
	if err != nil {
		if _, dErr := common.DiscordError(err); dErr != "" {
			return "Failed updating role, discord responded with: " + dErr, err
		} else {
			return "An unknown error occured updating the role", err
		}
	}

	_, err = common.BotSession.ChannelMessageSend(data.CS.ID(), "<@&"+discordgo.StrID(role.ID)+">")

	common.BotSession.GuildRoleEdit(data.GS.ID(), role.ID, role.Name, role.Color, role.Hoist, role.Permissions, false)
	return "", err
}

var cmdListRoles = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "ListRoles",
	Description: "List roles and their id's, and some other stuff on the server",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		out := ""

		data.GS.Lock()
		defer data.GS.Unlock()

		sort.Sort(dutil.Roles(data.GS.Guild.Roles))

		for _, r := range data.GS.Guild.Roles {
			me := r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionMentionEveryone != 0
			out += fmt.Sprintf("`%-25s: %-19s #%-6x  ME:%5t`\n", r.Name, r.ID, r.Color, me)
		}

		return out, nil
	},
}

var cmdWouldYouRather = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "WouldYouRather",
	Aliases:     []string{"wyr"},
	Description: "Get presented with 2 options.",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		q1, q2, err := WouldYouRather()
		if err != nil {
			return "Failed fetching the questions :(\n" + err.Error(), err
		}

		content := fmt.Sprintf("**Would you rather** (*<http://either.io>*)\n🇦 %s\n **OR**\n🇧 %s", q1, q2)
		msg, err := common.BotSession.ChannelMessageSend(data.Msg.ChannelID, content)
		if err != nil {
			return "Seomthing went wrong", err
		}

		common.BotSession.MessageReactionAdd(data.Msg.ChannelID, msg.ID, "🇦")
		err = common.BotSession.MessageReactionAdd(data.Msg.ChannelID, msg.ID, "🇧")
		if err != nil {
			_, dError := common.DiscordError(err)
			return "Failed adding reaction\n" + dError, err
		}

		return "", nil
	},
}

var cmdDefine = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Define",
	Aliases:      []string{"df"},
	Description:  "Look up an urban dictionary definition",
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Topic", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		qResp, err := urbandictionary.Query(data.Args[0].Str())
		if err != nil {
			return "Failed querying :(", err
		}

		if len(qResp.Results) < 1 {
			return "No result :(", nil
		}

		result := qResp.Results[0]

		cmdResp := fmt.Sprintf("**%s**: %s\n*%s*\n*(<%s>)*", result.Word, result.Definition, result.Example, result.Permalink)
		if len(qResp.Results) > 1 {
			cmdResp += fmt.Sprintf(" *%d more results*", len(qResp.Results)-1)
		}

		return cmdResp, nil
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
	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "Gateway (http send -> gateway receive time): "+taken.String())
	httpPing := time.Since(started)

	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "HTTP API (Edit Msg): "+httpPing.String()+"\nGateway: "+taken.String())
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

func WouldYouRather() (q1 string, q2 string, err error) {
	req, err := http.NewRequest("GET", "http://either.io/", nil)
	if err != nil {
		panic(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return
	}

	r1 := doc.Find("div.result.result-1 > .option-text")
	r2 := doc.Find("div.result.result-2 > .option-text")

	if len(r1.Nodes) < 1 || len(r2.Nodes) < 1 {
		return "", "", errors.New("Failed finding questions, format may have changed.")
	}

	q1 = r1.Nodes[0].FirstChild.Data
	q2 = r2.Nodes[0].FirstChild.Data
	return
}
