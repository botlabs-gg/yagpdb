package templates

import (
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/template"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"github.com/pkg/errors"
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

		// string manipulation
		"joinStr":   joinStrings,
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"slice":     slice,
		"urlescape": url.PathEscape,
		"split":     strings.Split,
		"title":     strings.Title,

		// math
		"add":        add,
		"mult":       tmplMult,
		"div":        tmplDiv,
		"mod":        tmplMod,
		"fdiv":       tmplFDiv,
		"round":      tmplRound,
		"roundCeil":  tmplRoundCeil,
		"roundFloor": tmplRoundFloor,
		"roundEven":  tmplRoundEven,

		// misc
		"dict":   Dictionary,
		"sdict":  StringKeyDictionary,
		"cembed": CreateEmbed,
		"cslice": CreateSlice,

		"formatTime":  tmplFormatTime,
		"json":        tmplJson,
		"in":          in,
		"inFold":      inFold,
		"roleAbove":   roleIsAbove,
		"adjective":   common.RandomAdjective,
		"randInt":     randInt,
		"shuffle":     shuffle,
		"seq":         sequence,
		"currentTime": tmplCurrentTime,

		"escapeHere":         tmplEscapeHere,
		"escapeEveryone":     tmplEscapeEveryone,
		"escapeEveryoneHere": tmplEscapeEveryoneHere,

		"humanizeDurationHours":   tmplHumanizeDurationHours,
		"humanizeDurationMinutes": tmplHumanizeDurationMinutes,
		"humanizeDurationSeconds": tmplHumanizeDurationSeconds,
		"humanizeTimeSinceDays":   tmplHumanizeTimeSinceDays,
	}

	contextSetupFuncs = []ContextSetupFunc{
		baseContextFuncs,
	}
)

var logger = common.GetFixedPrefixLogger("templates")

func TODO() {}

type ContextSetupFunc func(ctx *Context)

func RegisterSetupFunc(f ContextSetupFunc) {
	contextSetupFuncs = append(contextSetupFuncs, f)
}

// set by the premium package to return wether this guild is premium or not
var GuildPremiumFunc func(guildID int64) (bool, error)

type Context struct {
	Name string
	GS   *dstate.GuildState
	CS   *dstate.ChannelState

	MS  *dstate.MemberState
	Msg *discordgo.Message

	BotUser *discordgo.User

	ContextFuncs map[string]interface{}
	Data         map[string]interface{}

	MentionEveryone  bool
	MentionHere      bool
	MentionRoles     []int64
	MentionRoleNames []string

	DelResponse bool

	DelResponseDelay int

	Counters map[string]int

	EmebdsToSend []*discordgo.MessageEmbed

	AddResponseReactionNames []string

	FixedOutput string

	secondsSlept int

	IsPremium bool

	RegexCache map[string]*regexp.Regexp
}

func NewContext(gs *dstate.GuildState, cs *dstate.ChannelState, ms *dstate.MemberState) *Context {
	ctx := &Context{
		GS: gs,
		CS: cs,
		MS: ms,

		BotUser: common.BotUser,

		ContextFuncs: make(map[string]interface{}),
		Data:         make(map[string]interface{}),
		Counters:     make(map[string]int),
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

	if c.CS != nil {
		channel := c.CS.Copy(false)
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

func (c *Context) Execute(source string) (string, error) {
	if c.Msg == nil {
		// Construct a fake message
		c.Msg = new(discordgo.Message)
		c.Msg.Author = c.BotUser
		if c.CS != nil {
			c.Msg.ChannelID = c.CS.ID
		} else {
			// This may fail in some cases
			c.Msg.ChannelID = c.GS.ID
		}
		if c.GS != nil {
			c.Msg.GuildID = c.GS.ID
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

	var buf bytes.Buffer
	w := LimitWriter(&buf, 25000)

	started := time.Now()
	err = parsed.Execute(w, c.Data)

	dur := time.Since(started)
	if c.FixedOutput != "" {
		result := common.EscapeSpecialMentionsConditional(c.FixedOutput, c.MentionEveryone, c.MentionHere, c.MentionRoles)
		return result, nil
	}

	result := common.EscapeSpecialMentionsConditional(buf.String(), c.MentionEveryone, c.MentionHere, c.MentionRoles)
	if err != nil {
		if err == io.ErrShortWrite {
			err = errors.New("response grew too big (>25k)")
		}

		return result, errors.WithMessage(err, "Failed executing template (dur = "+dur.String()+")")
	}

	return result, nil
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
		out += "`" + common.EscapeSpecialMentions(err.Error()) + "`"
	}

	if strings.TrimSpace(out) != "" {
		_, err := common.BotSession.ChannelMessageSend(channelID, out)
		return err
	}

	return nil
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

	if c.CS != nil {
		f = f.WithField("channel", c.CS.ID)
	}

	return f
}

func baseContextFuncs(c *Context) {
	// message functions
	c.ContextFuncs["sendDM"] = c.tmplSendDM
	c.ContextFuncs["sendMessage"] = c.tmplSendMessage(true, false)
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
	c.ContextFuncs["getMessage"] = c.tmplGetMessage
	c.ContextFuncs["getMember"] = c.tmplGetMember
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

			bot.MessageDeleteQueue.DeleteMessages(channelID, messageID)
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
