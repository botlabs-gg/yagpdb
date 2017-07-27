package templates

import (
	"bytes"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"strings"
	"text/template"
)

var (
	StandardFuncMap = map[string]interface{}{
		"dict":      Dictionary,
		"in":        in,
		"title":     strings.Title,
		"add":       add,
		"roleAbove": roleIsAbove,
		"adjective": common.RandomAdjective,
		"randInt":   randInt,
		"shuffle":   shuffle,
		"seq":       sequence,
		"joinStr":   joinStrings,
		"lower":     strings.ToLower,
	}

	contextSetupFuncs []ContextSetupFunc
)

type ContextSetupFunc func(ctx *Context)

func RegisterSetupFunc(f ContextSetupFunc) {
	contextSetupFuncs = append(contextSetupFuncs, f)
}

type Context struct {
	GS *dstate.GuildState
	CS *dstate.ChannelState

	Member *discordgo.Member
	Msg    *discordgo.Message

	BotUser *discordgo.User

	ContextFuncs map[string]interface{}
	Data         map[string]interface{}
	Redis        *redis.Client
}

func NewContext(botUser *discordgo.User, gs *dstate.GuildState, cs *dstate.ChannelState, member *discordgo.Member) *Context {
	ctx := &Context{
		GS: gs,
		CS: cs,

		BotUser: botUser,
		Member:  member,

		ContextFuncs: make(map[string]interface{}),
		Data:         make(map[string]interface{}),
	}

	ctx.setupContextFuncs()

	return ctx
}

func (c *Context) setupContextFuncs() {
	for _, f := range contextSetupFuncs {
		f(c)
	}
}

func (c *Context) setupBaseData() {

	if c.GS != nil {
		guild := c.GS.LightCopy(false)
		c.Data["Guild"] = guild
		c.Data["Server"] = guild
		c.Data["server"] = guild
	}

	if c.CS != nil {
		channel := c.CS.Copy(false, false)
		c.Data["Channel"] = channel
		c.Data["channel"] = channel
	}

	if c.Member != nil {
		c.Data["Member"] = c.Member
		c.Data["User"] = c.Member.User
		c.Data["user"] = c.Member.User
	}
}

func (c *Context) Execute(redisClient *redis.Client, source string) (string, error) {
	if c.Msg == nil {
		// Construct a fake message
		c.Msg = new(discordgo.Message)
		c.Msg.Author = c.BotUser
		c.Msg.ChannelID = c.GS.ID()
	}

	if c.GS != nil {
		c.GS.RLock()
	}
	c.setupBaseData()
	if c.GS != nil {
		c.GS.RUnlock()
	}

	c.Redis = redisClient

	tmpl := template.New("")
	tmpl.Funcs(StandardFuncMap)
	tmpl.Funcs(c.ContextFuncs)

	parsed, err := tmpl.Parse(source)
	if err != nil {
		return "", errors.WithMessage(err, "Failed parsing template")
	}

	var buf bytes.Buffer
	err = parsed.Execute(&buf, c.Data)
	return buf.String(), errors.WithMessage(err, "Failed execuing template")
}
