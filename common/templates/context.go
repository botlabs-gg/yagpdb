package templates

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/template"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"github.com/sirupsen/logrus"
)

var (
	StandardFuncMap = map[string]interface{}{
		// conversion functions
		"str":        ToString,
		"toString":   ToString, // don't ask why we have 2 of these
		"toInt":      tmplToInt,
		"toInt64":    ToInt64,
		"toFloat":    ToFloat64,
		"toDuration": ToDuration,
		"toRune":     ToRune,
		"toByte":     ToByte,

		// string manipulation
		"joinStr":   joinStrings,
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"slice":     slice,
		"urlescape": url.PathEscape,
		"split":     strings.Split,
		"title":     strings.Title,

		// math
		"add":               add,
		"sub":               tmplSub,
		"mult":              tmplMult,
		"div":               tmplDiv,
		"mod":               tmplMod,
		"fdiv":              tmplFDiv,
		"sqrt":              tmplSqrt,
		"pow":               tmplPow,
		"log":               tmplLog,
		"round":             tmplRound,
		"roundCeil":         tmplRoundCeil,
		"roundFloor":        tmplRoundFloor,
		"roundEven":         tmplRoundEven,
		"humanizeThousands": tmplHumanizeThousands,

		// misc
		"dict":               Dictionary,
		"sdict":              StringKeyDictionary,
		"cembed":             CreateEmbed,
		"cslice":             CreateSlice,
		"complexMessage":     CreateMessageSend,
		"complexMessageEdit": CreateMessageEdit,

		"formatTime":  tmplFormatTime,
		"json":        tmplJson,
		"in":          in,
		"inFold":      inFold,
		"roleAbove":   roleIsAbove,
		"adjective":   common.RandomAdjective,
		"noun":        common.RandomNoun,
		"randInt":     randInt,
		"shuffle":     shuffle,
		"seq":         sequence,
		"currentTime": tmplCurrentTime,
		"newDate":     tmplNewDate,

		"escapeHere": func(s string) (string, error) {
			return "", errors.New("function is removed in favor of better direct control over mentions, join support server and read the announcements for more info.")
		},
		"escapeEveryone": func(s string) (string, error) {
			return "", errors.New("function is removed in favor of better direct control over mentions, join support server and read the announcements for more info.")
		},
		"escapeEveryoneHere": func(s string) (string, error) {
			return "", errors.New("function is removed in favor of better direct control over mentions, join support server and read the announcements for more info.")
		},

		"humanizeDurationHours":   tmplHumanizeDurationHours,
		"humanizeDurationMinutes": tmplHumanizeDurationMinutes,
		"humanizeDurationSeconds": tmplHumanizeDurationSeconds,
		"humanizeTimeSinceDays":   tmplHumanizeTimeSinceDays,
	}

	contextSetupFuncs = []ContextSetupFunc{}
)

var logger = common.GetFixedPrefixLogger("templates")

func TODO() {}

type ContextSetupFunc func(ctx *Context)

func RegisterSetupFunc(f ContextSetupFunc) {
	contextSetupFuncs = append(contextSetupFuncs, f)
}

func init() {
	RegisterSetupFunc(baseContextFuncs)
}

// set by the premium package to return wether this guild is premium or not
var GuildPremiumFunc func(guildID int64) (bool, error)

type Context struct {
	Name string

	GS      *dstate.GuildState
	MS      *dstate.MemberState
	Msg     *discordgo.Message
	BotUser *discordgo.User

	ContextFuncs map[string]interface{}
	Data         map[string]interface{}
	Counters     map[string]int

	FixedOutput  string
	secondsSlept int

	IsPremium bool

	RegexCache map[string]*regexp.Regexp

	CurrentFrame *contextFrame
}

type contextFrame struct {
	CS *dstate.ChannelState

	MentionEveryone bool
	MentionHere     bool
	MentionRoles    []int64

	DelResponse bool

	DelResponseDelay         int
	EmebdsToSend             []*discordgo.MessageEmbed
	AddResponseReactionNames []string

	isNestedTemplate bool
	parsedTemplate   *template.Template
	SendResponseInDM bool
}

func NewContext(gs *dstate.GuildState, cs *dstate.ChannelState, ms *dstate.MemberState) *Context {
	ctx := &Context{
		GS: gs,
		MS: ms,

		BotUser: common.BotUser,

		ContextFuncs: make(map[string]interface{}),
		Data:         make(map[string]interface{}),
		Counters:     make(map[string]int),

		CurrentFrame: &contextFrame{
			CS: cs,
		},
	}

	if gs != nil && GuildPremiumFunc != nil {
		ctx.IsPremium, _ = GuildPremiumFunc(gs.ID)
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
		guild := c.GS.DeepCopy(false, true, true, false)
		c.Data["Guild"] = guild
		c.Data["Server"] = guild
		c.Data["server"] = guild
	}

	if c.CurrentFrame.CS != nil {
		channel := CtxChannelFromCS(c.CurrentFrame.CS)
		c.Data["Channel"] = channel
		c.Data["channel"] = channel
	}

	if c.MS != nil {
		c.Data["Member"] = c.MS.DGoCopy()
		c.Data["User"] = c.MS.DGoUser()
		c.Data["user"] = c.Data["User"]
	}

	c.Data["TimeSecond"] = time.Second
	c.Data["TimeMinute"] = time.Minute
	c.Data["TimeHour"] = time.Hour
	c.Data["UnixEpoch"] = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Data["DiscordEpoch"] = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Data["IsPremium"] = c.IsPremium
}

func (c *Context) Parse(source string) (*template.Template, error) {
	tmpl := template.New(c.Name)
	tmpl.Funcs(StandardFuncMap)
	tmpl.Funcs(c.ContextFuncs)

	parsed, err := tmpl.Parse(source)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

const (
	MaxOpsNormal  = 1000000
	MaxOpsPremium = 2500000
)

func (c *Context) Execute(source string) (string, error) {
	if c.Msg == nil {
		// Construct a fake message
		c.Msg = new(discordgo.Message)
		c.Msg.Author = c.BotUser
		if c.CurrentFrame.CS != nil {
			c.Msg.ChannelID = c.CurrentFrame.CS.ID
		} else {
			// This may fail in some cases
			c.Msg.ChannelID = c.GS.ID
		}
		if c.GS != nil {
			c.Msg.GuildID = c.GS.ID

			member, err := bot.GetMember(c.GS.ID, c.BotUser.ID)
			if err != nil {
				return "", errors.WithMessage(err, "ctx.Execute")
			}

			c.Msg.Member = member.DGoCopy()
		}

	}

	if c.GS != nil {
		c.GS.RLock()
	}
	c.setupBaseData()
	if c.GS != nil {
		c.GS.RUnlock()
	}

	parsed, err := c.Parse(source)
	if err != nil {
		return "", errors.WithMessage(err, "Failed parsing template")
	}
	c.CurrentFrame.parsedTemplate = parsed

	return c.executeParsed()
}

func (c *Context) executeParsed() (string, error) {
	parsed := c.CurrentFrame.parsedTemplate
	if c.IsPremium {
		parsed = parsed.MaxOps(MaxOpsPremium)
	} else {
		parsed = parsed.MaxOps(MaxOpsNormal)
	}

	var buf bytes.Buffer
	w := LimitWriter(&buf, 25000)

	started := time.Now()
	err := parsed.Execute(w, c.Data)

	dur := time.Since(started)
	if c.FixedOutput != "" {
		return c.FixedOutput, nil
	}

	result := buf.String()
	if err != nil {
		if err == io.ErrShortWrite {
			err = errors.New("response grew too big (>25k)")
		}

		return result, errors.WithMessage(err, "Failed executing template (dur = "+dur.String()+")")
	}

	return result, nil
}

// creates a new context frame and returns the old one
func (c *Context) newContextFrame(cs *dstate.ChannelState) *contextFrame {
	old := c.CurrentFrame
	c.CurrentFrame = &contextFrame{
		CS:               cs,
		isNestedTemplate: true,
	}

	return old
}

func (c *Context) ExecuteAndSendWithErrors(source string, channelID int64) error {
	out, err := c.Execute(source)

	if utf8.RuneCountInString(out) > 2000 {
		out = "Template output for " + c.Name + " was longer than 2k (contact an admin on the server...)"
	}

	// deal with the results
	if err != nil {
		logger.WithField("guild", c.GS.ID).WithError(err).Error("Error executing template: " + c.Name)
		out += "\nAn error caused the execution of the custom command template to stop:\n"
		out += "`" + err.Error() + "`"
	}

	c.SendResponse(out)

	return nil
}

func (c *Context) MessageSend(content string) *discordgo.MessageSend {
	parse := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if c.CurrentFrame.MentionEveryone || c.CurrentFrame.MentionHere {
		parse = append(parse, discordgo.AllowedMentionTypeEveryone)
	}

	return &discordgo.MessageSend{
		Content: content,
		AllowedMentions: discordgo.AllowedMentions{
			Parse: parse,
			Roles: c.CurrentFrame.MentionRoles,
		},
	}
}

// SendResponse sends the response and handles reactions and the like
func (c *Context) SendResponse(content string) (*discordgo.Message, error) {
	channelID := int64(0)

	if !c.CurrentFrame.SendResponseInDM {
		if c.CurrentFrame.CS == nil {
			return nil, nil
		}

		if !bot.BotProbablyHasPermissionGS(c.GS, c.CurrentFrame.CS.ID, discordgo.PermissionSendMessages) {
			// don't bother sending the response if we dont have perms
			return nil, nil
		}

		channelID = c.CurrentFrame.CS.ID
	} else {
		if c.CurrentFrame.CS != nil && c.CurrentFrame.CS.Type == discordgo.ChannelTypeDM {
			channelID = c.CurrentFrame.CS.ID
		} else {
			privChannel, err := common.BotSession.UserChannelCreate(c.MS.ID)
			if err != nil {
				return nil, err
			}
			channelID = privChannel.ID
		}
	}

	for _, v := range c.CurrentFrame.EmebdsToSend {
		common.BotSession.ChannelMessageSendEmbed(channelID, v)
	}

	if strings.TrimSpace(content) == "" || (c.CurrentFrame.DelResponse && c.CurrentFrame.DelResponseDelay < 1) {
		// no point in sending the response if it gets deleted immedietely
		return nil, nil
	}

	m, err := common.BotSession.ChannelMessageSendComplex(channelID, c.MessageSend(content))
	if err != nil {
		logger.WithError(err).Error("Failed sending message")
	} else {
		if c.CurrentFrame.DelResponse {
			MaybeScheduledDeleteMessage(c.GS.ID, channelID, m.ID, c.CurrentFrame.DelResponseDelay)
		}

		if len(c.CurrentFrame.AddResponseReactionNames) > 0 {
			go func(frame *contextFrame) {
				for _, v := range frame.AddResponseReactionNames {
					common.BotSession.MessageReactionAdd(m.ChannelID, m.ID, v)
				}
			}(c.CurrentFrame)
		}
	}

	return m, nil
}

// IncreaseCheckCallCounter Returns true if key is above the limit
func (c *Context) IncreaseCheckCallCounter(key string, limit int) bool {
	current, ok := c.Counters[key]
	if !ok {
		current = 0
	}
	current++

	c.Counters[key] = current

	return current > limit
}

// IncreaseCheckCallCounter Returns true if key is above the limit
func (c *Context) IncreaseCheckCallCounterPremium(key string, normalLimit, premiumLimit int) bool {
	current, ok := c.Counters[key]
	if !ok {
		current = 0
	}
	current++

	c.Counters[key] = current

	if c.IsPremium {
		return current > premiumLimit
	}

	return current > normalLimit
}

func (c *Context) IncreaseCheckGenericAPICall() bool {
	return c.IncreaseCheckCallCounter("api_call", 100)
}

func (c *Context) IncreaseCheckStateLock() bool {
	return c.IncreaseCheckCallCounter("state_lock", 500)
}

func (c *Context) LogEntry() *logrus.Entry {
	f := logger.WithFields(logrus.Fields{
		"guild": c.GS.ID,
		"name":  c.Name,
	})

	if c.MS != nil {
		f = f.WithField("user", c.MS.ID)
	}

	if c.CurrentFrame.CS != nil {
		f = f.WithField("channel", c.CurrentFrame.CS.ID)
	}

	return f
}

func baseContextFuncs(c *Context) {
	// message functions
	c.ContextFuncs["sendDM"] = c.tmplSendDM
	c.ContextFuncs["sendMessage"] = c.tmplSendMessage(true, false)
	c.ContextFuncs["sendTemplate"] = c.tmplSendTemplate
	c.ContextFuncs["sendTemplateDM"] = c.tmplSendTemplateDM
	c.ContextFuncs["sendMessageRetID"] = c.tmplSendMessage(true, true)
	c.ContextFuncs["sendMessageNoEscape"] = c.tmplSendMessage(false, false)
	c.ContextFuncs["sendMessageNoEscapeRetID"] = c.tmplSendMessage(false, true)
	c.ContextFuncs["editMessage"] = c.tmplEditMessage(true)
	c.ContextFuncs["editMessageNoEscape"] = c.tmplEditMessage(false)

	// Mentions
	c.ContextFuncs["mentionEveryone"] = c.tmplMentionEveryone
	c.ContextFuncs["mentionHere"] = c.tmplMentionHere
	c.ContextFuncs["mentionRoleName"] = c.tmplMentionRoleName
	c.ContextFuncs["mentionRoleID"] = c.tmplMentionRoleID

	// Role functions
	c.ContextFuncs["hasRoleName"] = c.tmplHasRoleName
	c.ContextFuncs["hasRoleID"] = c.tmplHasRoleID

	c.ContextFuncs["addRoleID"] = c.tmplAddRoleID
	c.ContextFuncs["removeRoleID"] = c.tmplRemoveRoleID

	c.ContextFuncs["giveRoleID"] = c.tmplGiveRoleID
	c.ContextFuncs["giveRoleName"] = c.tmplGiveRoleName

	c.ContextFuncs["takeRoleID"] = c.tmplTakeRoleID
	c.ContextFuncs["takeRoleName"] = c.tmplTakeRoleName

	c.ContextFuncs["targetHasRoleID"] = c.tmplTargetHasRoleID
	c.ContextFuncs["targetHasRoleName"] = c.tmplTargetHasRoleName

	c.ContextFuncs["deleteResponse"] = c.tmplDelResponse
	c.ContextFuncs["deleteTrigger"] = c.tmplDelTrigger
	c.ContextFuncs["deleteMessage"] = c.tmplDelMessage
	c.ContextFuncs["deleteMessageReaction"] = c.tmplDelMessageReaction
	c.ContextFuncs["deleteAllMessageReactions"] = c.tmplDelAllMessageReactions
	c.ContextFuncs["getMessage"] = c.tmplGetMessage
	c.ContextFuncs["getMember"] = c.tmplGetMember
	c.ContextFuncs["getChannel"] = c.tmplGetChannel
	c.ContextFuncs["addReactions"] = c.tmplAddReactions
	c.ContextFuncs["addResponseReactions"] = c.tmplAddResponseReactions
	c.ContextFuncs["addMessageReactions"] = c.tmplAddMessageReactions

	c.ContextFuncs["currentUserCreated"] = c.tmplCurrentUserCreated
	c.ContextFuncs["currentUserAgeHuman"] = c.tmplCurrentUserAgeHuman
	c.ContextFuncs["currentUserAgeMinutes"] = c.tmplCurrentUserAgeMinutes
	c.ContextFuncs["sleep"] = c.tmplSleep
	c.ContextFuncs["reFind"] = c.reFind
	c.ContextFuncs["reFindAll"] = c.reFindAll
	c.ContextFuncs["reFindAllSubmatches"] = c.reFindAllSubmatches
	c.ContextFuncs["reReplace"] = c.reReplace

	c.ContextFuncs["editChannelTopic"] = c.tmplEditChannelTopic
	c.ContextFuncs["editChannelName"] = c.tmplEditChannelName
	c.ContextFuncs["onlineCount"] = c.tmplOnlineCount
	c.ContextFuncs["onlineCountBots"] = c.tmplOnlineCountBots
	c.ContextFuncs["editNickname"] = c.tmplEditNickname
}

type limitedWriter struct {
	W io.Writer
	N int64
}

func (l *limitedWriter) Write(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, io.ErrShortWrite
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
		err = io.ErrShortWrite
	}
	n, er := l.W.Write(p)
	if er != nil {
		err = er
	}
	l.N -= int64(n)
	return n, err
}

// LimitWriter works like io.LimitReader. It writes at most n bytes
// to the underlying Writer. It returns io.ErrShortWrite if more than n
// bytes are attempted to be written.
func LimitWriter(w io.Writer, n int64) io.Writer {
	return &limitedWriter{W: w, N: n}
}

func MaybeScheduledDeleteMessage(guildID, channelID, messageID int64, delaySeconds int) {
	if delaySeconds > 10 {
		err := scheduledevents2.ScheduleDeleteMessages(guildID, channelID, time.Now().Add(time.Second*time.Duration(delaySeconds)), messageID)
		if err != nil {
			logger.WithError(err).Error("failed scheduling message deletion")
		}
	} else {
		go func() {
			if delaySeconds > 0 {
				time.Sleep(time.Duration(delaySeconds) * time.Second)
			}

			bot.MessageDeleteQueue.DeleteMessages(guildID, channelID, messageID)
		}()
	}
}

type SDict map[string]interface{}

func (d SDict) Set(key string, value interface{}) string {
	d[key] = value
	return ""
}

func (d SDict) Get(key string) interface{} {
	return d[key]
}

func (d SDict) Del(key string) string {
	delete(d, key)
	return ""
}

type Slice []interface{}

func (s Slice) Append(item interface{}) (interface{}, error) {
	if len(s)+1 > 10000 {
		return nil, errors.New("resulting slice exceeds slice size limit")
	}

	switch v := item.(type) {
	case nil:
		result := reflect.Append(reflect.ValueOf(&s).Elem(), reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem()))
		return result.Interface(), nil
	default:
		result := reflect.Append(reflect.ValueOf(&s).Elem(), reflect.ValueOf(v))
		return result.Interface(), nil
	}

}

func (s Slice) Set(index int, item interface{}) (string, error) {
	if index >= len(s) {
		return "", errors.New("Index out of bounds")
	}

	s[index] = item
	return "", nil
}

func (s Slice) AppendSlice(slice interface{}) (interface{}, error) {
	val := reflect.ValueOf(slice)
	switch val.Kind() {
	case reflect.Slice, reflect.Array:
	// this is valid

	default:
		return nil, errors.New("value passed is not an array or slice")
	}

	if len(s)+val.Len() > 10000 {
		return nil, errors.New("resulting slice exceeds slice size limit")
	}

	result := reflect.ValueOf(&s).Elem()
	for i := 0; i < val.Len(); i++ {
		switch v := val.Index(i).Interface().(type) {
		case nil:
			result = reflect.Append(result, reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem()))

		default:
			result = reflect.Append(result, reflect.ValueOf(v))
		}
	}

	return result.Interface(), nil
}

func (s Slice) StringSlice(flag ...bool) interface{} {
	strict := false
	if len(flag) > 0 {
		strict = flag[0]
	}

	StringSlice := make([]string, 0, len(s))

	for _, Sliceval := range s {
		switch t := Sliceval.(type) {
		case string:
			StringSlice = append(StringSlice, t)

		case fmt.Stringer:
			if strict {
				return nil
			}
			StringSlice = append(StringSlice, t.String())

		default:
			if strict {
				return nil
			}
		}
	}

	return StringSlice
}
