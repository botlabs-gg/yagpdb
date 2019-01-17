package templates

import (
	"bytes"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/template"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"strings"
	"time"
)

var (
	StandardFuncMap = map[string]interface{}{
		"dict":                  Dictionary,
		"sdict":                 StringKeyDictionary,
		"cembed":                CreateEmbed,
		"json":                  tmplJson,
		"in":                    in,
		"inFold":                inFold,
		"title":                 strings.Title,
		"add":                   add,
		"roleAbove":             roleIsAbove,
		"adjective":             common.RandomAdjective,
		"randInt":               randInt,
		"shuffle":               shuffle,
		"seq":                   sequence,
		"joinStr":               joinStrings,
		"str":                   str,
		"lower":                 strings.ToLower,
		"upper":                 strings.ToUpper,
		"toString":              ToString,
		"toInt":                 tmplToInt,
		"toInt64":               ToInt64,
		"formatTime":            tmplFormatTime,
		"slice":                 slice,
		"cslice":                CreateSlice,
		"currentTime":           tmplCurrentTime,
		"escapeHere":            tmplEscapeHere,
		"escapeEveryone":        tmplEscapeEveryone,
		"escapeEveryoneHere":    tmplEscapeEveryoneHere,
		"humanizeDurationHours": tmplHumanizeDurationHours,
		"humanizeTimeSinceDays": tmplHumanizeTimeSinceDays,
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
	DelTrigger  bool

	DelTriggerDelay  int
	DelResponseDelay int

	Counters map[string]int

	EmebdsToSend []*discordgo.MessageEmbed

	AddResponseReactionNames []string

	FixedOutput string
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

	if c.MS != nil {
		c.Data["Member"] = c.MS.DGoCopy()
		c.Data["User"] = c.MS.DGoUser()
		c.Data["user"] = c.Data["User"]
	}
}

func (c *Context) Parse(source string) (*template.Template, error) {
	tmpl := template.New("")
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

	err = parsed.Execute(w, c.Data)

	if c.FixedOutput != "" {
		result := common.EscapeSpecialMentionsConditional(c.FixedOutput, c.MentionEveryone, c.MentionHere, c.MentionRoles)
		return result, nil
	}

	result := common.EscapeSpecialMentionsConditional(buf.String(), c.MentionEveryone, c.MentionHere, c.MentionRoles)
	if err != nil {
		if err == io.ErrShortWrite {
			err = errors.New("response grew too big (>25k)")
		}

		return result, errors.WithMessage(err, "Failed executing template")
	}

	return result, nil
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

func baseContextFuncs(c *Context) {
	c.ContextFuncs["sendDM"] = c.tmplSendDM
	c.ContextFuncs["sendMessage"] = c.tmplSendMessage(true, false)
	c.ContextFuncs["sendMessageRetID"] = c.tmplSendMessage(true, true)
	c.ContextFuncs["sendMessageNoEscape"] = c.tmplSendMessage(false, false)
	c.ContextFuncs["sendMessageNoEscapeRetID"] = c.tmplSendMessage(false, true)

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
	c.ContextFuncs["addReactions"] = c.tmplAddReactions
	c.ContextFuncs["addResponseReactions"] = c.tmplAddResponseReactions
	c.ContextFuncs["addMessageReactions"] = c.tmplAddMessageReactions

	c.ContextFuncs["currentUserCreated"] = c.tmplCurrentUserCreated
	c.ContextFuncs["currentUserAgeHuman"] = c.tmplCurrentUserAgeHuman
	c.ContextFuncs["currentUserAgeMinutes"] = c.tmplCurrentUserAgeMinutes
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
			logrus.WithError(err).Error("failed scheduling message deletion")
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
