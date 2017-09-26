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
		"str":       str,
		"lower":     strings.ToLower,
		"toString":  tmplToString,
		"toInt":     tmplToInt,
		"toInt64":   tmplToInt64,
	}

	contextSetupFuncs = []ContextSetupFunc{
		baseContextFuncs,
	}
)

func TODO() {}

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

	MentionEveryone  bool
	MentionHere      bool
	MentionRoles     []string
	MentionRoleNames []string

	SentDM bool
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

	result := common.EscapeSpecialMentions(buf.String())
	if err != nil {
		return result, errors.WithMessage(err, "Failed execuing template")
	}

	return c.ApplyPostResponseModifications(result), nil
}

func (c *Context) ApplyPostResponseModifications(resp string) string {
	resp += "\n"
	if c.MentionEveryone {
		resp += "@everyone "
	}
	if c.MentionHere {
		resp += "@here "
	}

	c.GS.RLock()
	for _, role := range c.GS.Guild.Roles {
		if common.ContainsStringSliceFold(c.MentionRoleNames, role.Name) || common.ContainsStringSlice(c.MentionRoles, role.ID) {
			resp += "<@&" + role.ID + "> "
		}
	}

	c.GS.RUnlock()
	return resp
}

func baseContextFuncs(c *Context) {
	c.ContextFuncs["sendDM"] = tmplSendDM(c)
	c.ContextFuncs["mentionEveryone"] = tmplMentionEveryone(c)
	c.ContextFuncs["mentionHere"] = tmplMentionHere(c)
	c.ContextFuncs["mentionRoleName"] = tmplMentionRoleName(c)
	c.ContextFuncs["mentionRoleID"] = tmplMentionRoleID(c)
	c.ContextFuncs["hasRoleName"] = tmplHasRoleName(c)
	c.ContextFuncs["hasRoleID"] = tmplHasRoleID(c)
}
