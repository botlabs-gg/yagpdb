package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/prefix"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/lib/template"
	"github.com/botlabs-gg/yagpdb/v2/web/discorddata"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
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
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"joinStr":      joinStrings,
		"lower":        strings.ToLower,
		"slice":        slice,
		"split":        strings.Split,
		"title":        strings.Title,
		"trimSpace":    strings.TrimSpace,
		"upper":        strings.ToUpper,
		"urlescape":    url.PathEscape,
		"urlunescape":  url.PathUnescape,
		"print":        withOutputLimit(fmt.Sprint, MaxStringLength),
		"println":      withOutputLimit(fmt.Sprintln, MaxStringLength),
		"printf":       withOutputLimitF(fmt.Sprintf, MaxStringLength),
		"sanitizeText": confusables.SanitizeText,

		// regexp
		"reQuoteMeta": regexp.QuoteMeta,

		// math
		"abs":        tmplAbs,
		"add":        add,
		"cbrt":       tmplCbrt,
		"div":        tmplDiv,
		"fdiv":       tmplFDiv,
		"log":        tmplLog,
		"mathConst":  tmplMathConstant,
		"max":        tmplMax,
		"min":        tmplMin,
		"mod":        tmplMod,
		"mult":       tmplMult,
		"pow":        tmplPow,
		"round":      tmplRound,
		"roundCeil":  tmplRoundCeil,
		"roundEven":  tmplRoundEven,
		"roundFloor": tmplRoundFloor,
		"sqrt":       tmplSqrt,
		"sub":        tmplSub,

		// bitwise ops
		"bitwiseAnd":        tmplBitwiseAnd,
		"bitwiseOr":         tmplBitwiseOr,
		"bitwiseXor":        tmplBitwiseXor,
		"bitwiseNot":        tmplBitwiseNot,
		"bitwiseAndNot":     tmplBitwiseAndNot,
		"bitwiseLeftShift":  tmplBitwiseLeftShift,
		"bitwiseRightShift": tmplBitwiseRightShift,

		// misc
		"humanizeThousands":  tmplHumanizeThousands,
		"dict":               Dictionary,
		"sdict":              StringKeyDictionary,
		"structToSdict":      StructToSdict,
		"componentBuilder":   CreateComponentBuilder,
		"cembed":             CreateEmbed,
		"cbutton":            CreateButton,
		"cmenu":              CreateSelectMenu,
		"cmodal":             CreateModal,
		"cslice":             CreateSlice,
		"complexMessage":     CreateMessageSend,
		"complexMessageEdit": CreateMessageEdit,
		"kindOf":             KindOf,

		"adjective":   common.RandomAdjective,
		"in":          in,
		"inFold":      inFold,
		"json":        tmplJson,
		"jsonToSdict": tmplJSONToSDict,
		"noun":        common.RandomNoun,
		"randInt":     randInt,
		"roleAbove":   roleIsAbove,
		"seq":         sequence,

		"shuffle": shuffle,
		"verb":    common.RandomVerb,

		// time functions
		"currentTime":     tmplCurrentTime,
		"parseTime":       tmplParseTime,
		"formatTime":      tmplFormatTime,
		"loadLocation":    time.LoadLocation,
		"newDate":         tmplNewDate,
		"snowflakeToTime": tmplSnowflakeToTime,
		"timestampToTime": tmplTimestampToTime,
		"weekNumber":      tmplWeekNumber,

		"humanizeDurationHours":   tmplHumanizeDurationHours,
		"humanizeDurationMinutes": tmplHumanizeDurationMinutes,
		"humanizeDurationSeconds": tmplHumanizeDurationSeconds,
		"humanizeTimeSinceDays":   tmplHumanizeTimeSinceDays,
	}

	contextSetupFuncs = []ContextSetupFunc{}
)

var logger = common.GetFixedPrefixLogger("templates")

type ContextSetupFunc func(ctx *Context)

func RegisterSetupFunc(f ContextSetupFunc) {
	contextSetupFuncs = append(contextSetupFuncs, f)
}

func init() {
	RegisterSetupFunc(baseContextFuncs)
	RegisterSetupFunc(interactionContextFuncs)

	msgpack.RegisterExt(1, (*SDict)(nil))
	msgpack.RegisterExt(2, (*Dict)(nil))
	msgpack.RegisterExt(3, (*Slice)(nil))
}

// set by the premium package to return wether this guild is premium or not
var GuildPremiumFunc func(guildID int64) (bool, error)

// Defines where a template was executed from to enable certain restrictions
type ExecutedFromType int

const (
	ExecutedFromStandard              ExecutedFromType = 0
	ExecutedFromJoin                  ExecutedFromType = 1
	ExecutedFromLeave                 ExecutedFromType = 2
	ExecutedFromEvalCC                ExecutedFromType = 3
	ExecutedFromCommandTemplate       ExecutedFromType = 4
	ExecutedFromNestedCommandTemplate ExecutedFromType = 5
)

type Context struct {
	Name string

	GS      *dstate.GuildSet
	MS      *dstate.MemberState
	Msg     *discordgo.Message
	BotUser *discordgo.User

	DisabledContextFuncs []string
	ContextFuncs         map[string]interface{}
	Data                 map[string]interface{}
	Counters             map[string]int

	FixedOutput  string
	secondsSlept int

	IsPremium bool

	RegexCache map[string]*regexp.Regexp

	CurrentFrame *ContextFrame

	ExecutedFrom ExecutedFromType

	contextFuncsAdded bool
}

type ContextFrame struct {
	CS *dstate.ChannelState

	MentionEveryone bool
	MentionHere     bool
	MentionRoles    []int64

	DelResponse       bool
	PublishResponse   bool
	EphemeralResponse bool

	DelResponseDelay         int
	EmbedsToSend             []*discordgo.MessageEmbed
	ComponentsToSend         []discordgo.TopLevelComponent
	AddResponseReactionNames []string

	isNestedTemplate bool
	parsedTemplate   *template.Template
	SendResponseInDM bool

	Interaction *CustomCommandInteraction
}

type CustomCommandInteraction struct {
	*discordgo.Interaction
	RespondedTo bool
	Deferred    bool
}

func NewContext(gs *dstate.GuildSet, cs *dstate.ChannelState, ms *dstate.MemberState) *Context {
	ctx := &Context{
		GS: gs,
		MS: ms,

		BotUser: common.BotUser,

		ContextFuncs: make(map[string]interface{}),
		Data:         make(map[string]interface{}),
		Counters:     make(map[string]int),

		CurrentFrame: &ContextFrame{
			CS: cs,
		},
	}

	if gs != nil && GuildPremiumFunc != nil {
		ctx.IsPremium, _ = GuildPremiumFunc(gs.ID)
	}

	return ctx
}

func (c *Context) setupContextFuncs() {
	for _, f := range contextSetupFuncs {
		f(c)
	}

	c.contextFuncsAdded = true
}

func (c *Context) setupBaseData() {

	if c.GS != nil {
		c.Data["Guild"] = c.GS
		c.Data["Server"] = c.GS
		c.Data["server"] = c.GS
		c.Data["ServerPrefix"] = prefix.GetPrefixIgnoreError(c.GS.ID)
	}

	if c.CurrentFrame.CS != nil {
		channel := CtxChannelFromCS(c.CurrentFrame.CS)
		c.Data["Channel"] = channel
		c.Data["channel"] = channel

		if parentID := common.ChannelOrThreadParentID(c.CurrentFrame.CS); parentID != c.CurrentFrame.CS.ID {
			c.Data["ChannelOrThreadParent"] = CtxChannelFromCS(c.GS.GetChannelOrThread(parentID))
		} else {
			c.Data["ChannelOrThreadParent"] = channel
		}
	}

	if c.MS != nil {
		c.Data["BotUser"] = common.BotUser
		c.Data["Member"] = c.MS.DgoMember()
		c.Data["User"] = &c.MS.User
		c.Data["user"] = c.Data["User"]
	}

	c.Data["DiscordEpoch"] = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Data["DomainRegex"] = common.DomainFinderRegex.String()
	c.Data["IsPremium"] = c.IsPremium
	c.Data["LinkRegex"] = common.LinkRegex.String()
	c.Data["TimeHour"] = time.Hour
	c.Data["TimeMinute"] = time.Minute
	c.Data["TimeSecond"] = time.Second
	c.Data["UnixEpoch"] = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	permNameToBit := make(map[string]int64)
	for _, p := range discordgo.AllPermissions {
		permNameToBit[discordgo.PermissionName(p)] = p
	}

	// for backward compatibility with previous versions
	permNameToBit["ReadMessages"] = discordgo.PermissionViewChannel
	permNameToBit["ManageEmojis"] = discordgo.PermissionManageGuildExpressions
	permNameToBit["ManageServer"] = discordgo.PermissionManageGuild
	permNameToBit["ViewAuditLogs"] = discordgo.PermissionViewAuditLog

	c.Data["Permissions"] = permNameToBit
}

func (c *Context) Parse(source string) (*template.Template, error) {
	if !c.contextFuncsAdded {
		c.setupContextFuncs()
	}

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
	MaxOpsNormal      = 1000000
	MaxOpsPremium     = 2500000
	MaxOpsEvalNormal  = 200000
	MaxOpsEvalPremium = 500000
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
			if bot.State != nil {
				member, err := bot.GetMember(c.GS.ID, c.BotUser.ID)
				if err != nil {
					return "", errors.WithMessage(err, "ctx.Execute")
				}
				c.Msg.Member = member.DgoMember()
			} else {
				member, err := discorddata.GetMember(c.GS.ID, c.BotUser.ID)
				if err != nil {
					return "", errors.WithMessage(err, "ctx.Execute")
				}
				c.Msg.Member = member
			}
		}
	}

	c.setupBaseData()

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
		if c.ExecutedFrom == ExecutedFromEvalCC {
			parsed = parsed.MaxOps(MaxOpsEvalPremium)
		}
	} else {
		parsed = parsed.MaxOps(MaxOpsNormal)
		if c.ExecutedFrom == ExecutedFromEvalCC {
			parsed = parsed.MaxOps(MaxOpsEvalNormal)
		}
	}

	var buf bytes.Buffer
	w := LimitWriter(&buf, 25000)

	// started := time.Now()
	err := parsed.Execute(w, c.Data)

	// dur := time.Since(started)
	if c.FixedOutput != "" {
		return c.FixedOutput, nil
	}

	result := buf.String()
	if err != nil {
		if err == io.ErrShortWrite {
			err = errors.New("response grew too big (>25k)")
		}

		return result, errors.WithMessage(err, "Failed executing template")
	}

	return result, nil
}

// creates a new context frame and returns the old one
func (c *Context) newContextFrame(cs *dstate.ChannelState) *ContextFrame {
	old := c.CurrentFrame
	c.CurrentFrame = &ContextFrame{
		CS:               cs,
		isNestedTemplate: true,
	}

	return old
}

func (c *Context) ExecuteAndSendWithErrors(source string, channelID int64) error {
	out, err := c.Execute(source)

	// trim whitespace for accurate character count
	out = strings.TrimSpace(out)

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
func (c *Context) SendResponse(content string) (m *discordgo.Message, err error) {
	channelID := int64(0)

	sendType := sendMessageGuildChannel
	if c.CurrentFrame.Interaction != nil {
		if c.CurrentFrame.Interaction.RespondedTo {
			sendType = sendMessageInteractionFollowup
			if c.CurrentFrame.Interaction.Deferred {
				sendType = sendMessageInteractionDeferred
			}
		} else {
			sendType = sendMessageInteractionResponse
		}
	} else if c.CurrentFrame.SendResponseInDM || (c.CurrentFrame.CS != nil && c.CurrentFrame.CS.IsPrivate()) {
		sendType = sendMessageDM
		if c.CurrentFrame.CS != nil && c.CurrentFrame.CS.Type == discordgo.ChannelTypeDM {
			channelID = c.CurrentFrame.CS.ID
		} else {
			privChannel, err := common.BotSession.UserChannelCreate(c.MS.User.ID)
			if err != nil {
				return nil, err
			}
			channelID = privChannel.ID
		}
	} else {
		if c.CurrentFrame.CS == nil {
			return nil, nil
		}

		if hasPerms, _ := bot.BotHasPermissionGS(c.GS, c.CurrentFrame.CS.ID, discordgo.PermissionSendMessages); !hasPerms {
			// don't bother sending the response if we dont have perms
			return nil, nil
		}

		channelID = c.CurrentFrame.CS.ID
	}

	msgSend := c.MessageSend("")
	var embeds []*discordgo.MessageEmbed
	embeds = append(embeds, c.CurrentFrame.EmbedsToSend...)
	msgSend.Embeds = embeds
	msgSend.Content = content
	if len(c.CurrentFrame.ComponentsToSend) > 0 {
		msgSend.Components = append(msgSend.Components, c.CurrentFrame.ComponentsToSend...)
		if len(msgSend.Components) > 5 {
			msgSend.Components = msgSend.Components[:5]
		}
	}
	if (len(msgSend.Embeds) == 0 && strings.TrimSpace(content) == "" && len(msgSend.Components) == 0) || (c.CurrentFrame.DelResponse && c.CurrentFrame.DelResponseDelay < 1) {
		// no point in sending the response if it gets deleted immedietely
		return nil, nil
	}
	if sendType == sendMessageDM {
		msgSend.Content = common.ReplaceServerInvites(msgSend.Content, 0, "[removed-server-invite]")
		msgSend.Components = []discordgo.TopLevelComponent{
			discordgo.ActionsRow{
				Components: []discordgo.InteractiveComponent{
					discordgo.Button{
						Label:    "Show Server Info",
						Style:    discordgo.PrimaryButton,
						Emoji:    &discordgo.ComponentEmoji{Name: "📬"},
						CustomID: fmt.Sprintf("DM_%d", c.GS.ID),
					},
				},
			},
		}
	}

	if c.CurrentFrame.EphemeralResponse {
		msgSend.Flags |= discordgo.MessageFlagsEphemeral
	}

	var getErr error

	switch sendType {
	case sendMessageInteractionResponse:
		err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, c.CurrentFrame.Interaction.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:         msgSend.Content,
				Embeds:          msgSend.Embeds,
				AllowedMentions: &msgSend.AllowedMentions,
				Flags:           msgSend.Flags,
			},
		})
		if err == nil {
			c.CurrentFrame.Interaction.RespondedTo = true
			m, getErr = common.BotSession.GetOriginalInteractionResponse(common.BotApplication.ID, c.CurrentFrame.Interaction.Token)
		}
	case sendMessageInteractionFollowup:
		m, err = common.BotSession.CreateFollowupMessage(common.BotApplication.ID, c.CurrentFrame.Interaction.Token, &discordgo.WebhookParams{
			Content:         msgSend.Content,
			Embeds:          msgSend.Embeds,
			AllowedMentions: &msgSend.AllowedMentions,
			Flags:           int64(msgSend.Flags),
		})
	case sendMessageInteractionDeferred:
		m, err = common.BotSession.EditOriginalInteractionResponse(common.BotApplication.ID, c.CurrentFrame.Interaction.Token, &discordgo.WebhookParams{
			Content:         msgSend.Content,
			Embeds:          msgSend.Embeds,
			AllowedMentions: &msgSend.AllowedMentions,
			Flags:           int64(msgSend.Flags),
		})
		if err == nil {
			c.CurrentFrame.Interaction.Deferred = false
		}
	default:
		m, err = common.BotSession.ChannelMessageSendComplex(channelID, msgSend)
	}

	if err != nil {
		logger.WithError(err).Error("Failed sending message")
	} else if getErr != nil {
		logger.WithError(getErr).Error("Failed getting interaction response")
	} else {
		if c.CurrentFrame.DelResponse {
			var maybeToken string
			if c.CurrentFrame.Interaction != nil {
				maybeToken = c.CurrentFrame.Interaction.Token
			}
			MaybeScheduledDeleteMessage(c.GS.ID, channelID, m.ID, c.CurrentFrame.DelResponseDelay, maybeToken)
		}
		if !c.CurrentFrame.EphemeralResponse {
			if len(c.CurrentFrame.AddResponseReactionNames) > 0 {
				go func(frame *ContextFrame) {
					for _, v := range frame.AddResponseReactionNames {
						common.BotSession.MessageReactionAdd(m.ChannelID, m.ID, v)
					}
				}(c.CurrentFrame)
			}

			if c.CurrentFrame.PublishResponse && c.CurrentFrame.CS.Type == discordgo.ChannelTypeGuildNews {
				common.BotSession.ChannelMessageCrosspost(m.ChannelID, m.ID)
			}
		}
	}

	return
}

type sendMessageType uint

const (
	sendMessageGuildChannel sendMessageType = iota
	sendMessageDM
	sendMessageInteractionResponse
	sendMessageInteractionFollowup
	sendMessageInteractionDeferred
)

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
	if c.ExecutedFrom == ExecutedFromEvalCC {
		return c.IncreaseCheckCallCounter("api_call", 20)
	}
	return c.IncreaseCheckCallCounter("api_call", 100)
}

func (c *Context) LogEntry() *logrus.Entry {
	f := logger.WithFields(logrus.Fields{
		"guild": c.GS.ID,
		"name":  c.Name,
	})

	if c.MS != nil {
		f = f.WithField("user", c.MS.User.ID)
	}

	if c.CurrentFrame.CS != nil {
		f = f.WithField("channel", c.CurrentFrame.CS.ID)
	}

	return f
}

func (c *Context) addContextFunc(name string, f interface{}) {
	if !common.ContainsStringSlice(c.DisabledContextFuncs, name) {
		c.ContextFuncs[name] = f
	}
}

func baseContextFuncs(c *Context) {
	// Message functions
	c.addContextFunc("deleteMessage", c.tmplDelMessage)
	c.addContextFunc("deleteResponse", c.tmplDelResponse)
	c.addContextFunc("deleteTrigger", c.tmplDelTrigger)

	c.addContextFunc("editComponentMessage", c.tmplEditComponentsMessage(true))
	c.addContextFunc("editComponentMessageNoEscape", c.tmplEditComponentsMessage(false))
	c.addContextFunc("editMessage", c.tmplEditMessage(true))
	c.addContextFunc("editMessageNoEscape", c.tmplEditMessage(false))
	c.addContextFunc("getMessage", c.tmplGetMessage)
	c.addContextFunc("pinMessage", c.tmplPinMessage(false))
	c.addContextFunc("publishMessage", c.tmplPublishMessage)
	c.addContextFunc("publishResponse", c.tmplPublishResponse)
	c.addContextFunc("unpinMessage", c.tmplPinMessage(true))

	// Message send functions
	c.addContextFunc("sendDM", c.tmplSendDM)
	c.addContextFunc("sendComponentMessageRetID", c.tmplSendComponentsMessage(true, true))
	c.addContextFunc("sendComponentMessage", c.tmplSendComponentsMessage(true, false))
	c.addContextFunc("sendComponentMessageNoEscape", c.tmplSendComponentsMessage(false, false))
	c.addContextFunc("sendComponentMessageNoEscapeRetID", c.tmplSendComponentsMessage(false, true))
	c.addContextFunc("sendComponentMessageRetID", c.tmplSendComponentsMessage(true, true))
	c.addContextFunc("sendMessage", c.tmplSendMessage(true, false))
	c.addContextFunc("sendMessageNoEscape", c.tmplSendMessage(false, false))
	c.addContextFunc("sendMessageNoEscapeRetID", c.tmplSendMessage(false, true))
	c.addContextFunc("sendMessageRetID", c.tmplSendMessage(true, true))

	c.addContextFunc("sendTemplate", c.tmplSendTemplate)
	c.addContextFunc("sendTemplateDM", c.tmplSendTemplateDM)

	// Reaction functions
	c.addContextFunc("addMessageReactions", c.tmplAddMessageReactions)
	c.addContextFunc("addReactions", c.tmplAddReactions)
	c.addContextFunc("addResponseReactions", c.tmplAddResponseReactions)

	c.addContextFunc("deleteAllMessageReactions", c.tmplDelAllMessageReactions)
	c.addContextFunc("deleteMessageReaction", c.tmplDelMessageReaction)

	// Mention functions
	c.addContextFunc("mentionEveryone", c.tmplMentionEveryone)
	c.addContextFunc("mentionHere", c.tmplMentionHere)
	c.addContextFunc("mentionRole", c.tmplMentionRole)
	c.addContextFunc("mentionRoleName", c.tmplMentionRoleName)
	c.addContextFunc("mentionRoleID", c.tmplMentionRoleID)

	// Role functions
	c.addContextFunc("getRole", c.tmplGetRole)
	c.addContextFunc("getRoleID", c.tmplGetRoleID)
	c.addContextFunc("getRoleName", c.tmplGetRoleName)

	c.addContextFunc("hasRole", c.tmplHasRole)
	c.addContextFunc("hasRoleID", c.tmplHasRoleID)
	c.addContextFunc("hasRoleName", c.tmplHasRoleName)

	c.addContextFunc("targetHasRole", c.tmplTargetHasRole)
	c.addContextFunc("targetHasRoleID", c.tmplTargetHasRoleID)
	c.addContextFunc("targetHasRoleName", c.tmplTargetHasRoleName)

	c.addContextFunc("giveRole", c.tmplGiveRole)
	c.addContextFunc("giveRoleID", c.tmplGiveRoleID)
	c.addContextFunc("giveRoleName", c.tmplGiveRoleName)

	c.addContextFunc("addRole", c.tmplAddRole)
	c.addContextFunc("addRoleID", c.tmplAddRoleID)
	c.addContextFunc("addRoleName", c.tmplAddRoleName)

	c.addContextFunc("takeRole", c.tmplTakeRole)
	c.addContextFunc("takeRoleID", c.tmplTakeRoleID)
	c.addContextFunc("takeRoleName", c.tmplTakeRoleName)

	c.addContextFunc("removeRole", c.tmplRemoveRole)
	c.addContextFunc("removeRoleID", c.tmplRemoveRoleID)
	c.addContextFunc("removeRoleName", c.tmplRemoveRoleName)

	c.addContextFunc("setRoles", c.tmplSetRoles)

	// Permission functions
	c.addContextFunc("hasPermissions", c.tmplHasPermissions)
	c.addContextFunc("targetHasPermissions", c.tmplTargetHasPermissions)
	c.addContextFunc("getTargetPermissionsIn", c.tmplGetTargetPermissionsIn)

	// Channel functions
	c.addContextFunc("editChannelName", c.tmplEditChannelName)
	c.addContextFunc("editChannelTopic", c.tmplEditChannelTopic)

	c.addContextFunc("getChannel", c.tmplGetChannel)
	c.addContextFunc("getChannelPins", c.tmplGetChannelPins(false))
	c.addContextFunc("getChannelOrThread", c.tmplGetChannelOrThread)
	c.addContextFunc("getPinCount", c.tmplGetChannelPins(true))

	// Member functions
	c.addContextFunc("getMember", c.tmplGetMember)
	c.addContextFunc("getMemberVoiceState", c.tmplGetMemberVoiceState)
	c.addContextFunc("editNickname", c.tmplEditNickname)

	// Thread functions
	c.addContextFunc("addThreadMember", c.tmplThreadMemberAdd)
	c.addContextFunc("closeThread", c.tmplCloseThread)
	c.addContextFunc("createThread", c.tmplCreateThread)
	c.addContextFunc("deleteThread", c.tmplDeleteThread)
	c.addContextFunc("editThread", c.tmplEditThread)
	c.addContextFunc("getThread", c.tmplGetThread)
	c.addContextFunc("openThread", c.tmplOpenThread)
	c.addContextFunc("removeThreadMember", c.tmplThreadMemberRemove)

	// Forum functions
	c.addContextFunc("createForumPost", c.tmplCreateForumPost)
	c.addContextFunc("deleteForumPost", c.tmplDeleteThread)
	c.addContextFunc("pinForumPost", c.tmplPinForumPost(false))
	c.addContextFunc("unpinForumPost", c.tmplPinForumPost(true))

	// User functions
	c.addContextFunc("currentUserAgeHuman", c.tmplCurrentUserAgeHuman)
	c.addContextFunc("currentUserAgeMinutes", c.tmplCurrentUserAgeMinutes)
	c.addContextFunc("currentUserCreated", c.tmplCurrentUserCreated)

	// RegEx functions
	c.addContextFunc("reFind", c.reFind)
	c.addContextFunc("reFindAll", c.reFindAll)
	c.addContextFunc("reFindAllSubmatches", c.reFindAllSubmatches)
	c.addContextFunc("reReplace", c.reReplace)
	c.addContextFunc("reSplit", c.reSplit)

	// Miscellaneous functions
	c.addContextFunc("onlineCount", c.tmplOnlineCount)
	c.addContextFunc("onlineCountBots", c.tmplOnlineCountBots)

	c.addContextFunc("sleep", c.tmplSleep)
	c.addContextFunc("sort", c.tmplSort)
	c.addContextFunc("hash", c.tmplSha256)
	c.addContextFunc("decodeBase64", c.tmplDecodeBase64)
	c.addContextFunc("encodeBase64", c.tmplEncodeBase64)
}

type limitedWriter struct {
	W io.Writer
	N int64
	i int64
}

func (l *limitedWriter) Write(p []byte) (n int, err error) {
	noLeadingWhitespace := trimLeftSpace(p)
	if l.N == l.i {
		if len(noLeadingWhitespace) < 1 {
			return 0, nil
		} else {
			p = noLeadingWhitespace
		}
	}

	if l.N <= 0 {
		swErr := io.ErrShortWrite
		if len(noLeadingWhitespace) < 1 {
			swErr = nil
		}
		return 0, swErr
	}
	if int64(len(p)) > l.N {
		var cut []byte
		p, cut = p[0:l.N], p[l.N:]
		if len(bytes.TrimSpace(cut)) > 0 {
			err = io.ErrShortWrite
		}
	}
	n, er := l.W.Write(p)
	if er != nil {
		err = er
	}
	l.N -= int64(n)
	return n, err
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

func trimLeftSpace(s []byte) []byte {
	// Fast path for ASCII: look for the first ASCII non-space byte
	start := 0
	for ; start < len(s); start++ {
		c := s[start]
		if c >= utf8.RuneSelf {
			// If we run into a non-ASCII byte, fall back to the
			// slower unicode-aware method on the remaining bytes
			return bytes.TrimLeftFunc(s[start:], unicode.IsSpace)
		}
		if asciiSpace[c] == 0 {
			break
		}
	}

	return s[start:]
}

// LimitWriter works like io.LimitReader. It writes at most n bytes
// to the underlying Writer. It returns io.ErrShortWrite if more than n
// bytes are attempted to be written, unless those bytes are exclusively
// whitespace, in which case it will not write them and return without error.
// It will not write leading whitespace.
func LimitWriter(w io.Writer, n int64) io.Writer {
	return &limitedWriter{W: w, N: n, i: n}
}

func MaybeScheduledDeleteMessage(guildID, channelID, messageID int64, delaySeconds int, token string) {
	if delaySeconds > 10 && token == "" {
		err := scheduledevents2.ScheduleDeleteMessages(guildID, channelID, time.Now().Add(time.Second*time.Duration(delaySeconds)), messageID)
		if err != nil {
			logger.WithError(err).Error("failed scheduling message deletion")
		}
	} else {
		if delaySeconds > 10 {
			delaySeconds = 10
		}
		go func() {
			if delaySeconds > 0 {
				time.Sleep(time.Duration(delaySeconds) * time.Second)
			}

			if token != "" {
				if messageID != 0 {
					common.BotSession.DeleteFollowupMessage(common.BotApplication.ID, token, messageID)
				} else {
					common.BotSession.DeleteInteractionResponse(common.BotApplication.ID, token)
				}
				return
			}
			bot.MessageDeleteQueue.DeleteMessages(guildID, channelID, messageID)
		}()
	}
}

func isContainer(v interface{}) bool {
	rv, _ := indirect(reflect.ValueOf(v))
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return true
	default:
		return false
	}
}

// Cyclic value detection is modified from encoding/json/encode.go.
const startDetectingCyclesAfter = 250

type cyclicValueDetector struct {
	ptrLevel uint
	ptrSeen  map[interface{}]struct{}
}

func (c *cyclicValueDetector) Check(v reflect.Value) error {
	v, _ = indirect(v)
	switch v.Kind() {
	case reflect.Map:
		if c.ptrLevel++; c.ptrLevel > startDetectingCyclesAfter {
			ptr := v.Pointer()
			if _, ok := c.ptrSeen[ptr]; ok {
				return fmt.Errorf("encountered a cycle via %s", v.Type())
			}
			c.ptrSeen[ptr] = struct{}{}
		}

		it := v.MapRange()
		for it.Next() {
			if err := c.Check(it.Value()); err != nil {
				return err
			}
		}
		c.ptrLevel--
		return nil
	case reflect.Array, reflect.Slice:
		if c.ptrLevel++; c.ptrLevel > startDetectingCyclesAfter {
			ptr := struct {
				ptr uintptr
				len int
			}{v.Pointer(), v.Len()}
			if _, ok := c.ptrSeen[ptr]; ok {
				return fmt.Errorf("encountered a cycle via %s", v.Type())
			}
			c.ptrSeen[ptr] = struct{}{}
		}

		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			if err := c.Check(elem); err != nil {
				return err
			}
		}
		c.ptrLevel--
		return nil
	default:
		return nil
	}
}

func detectCyclicValue(v interface{}) error {
	c := &cyclicValueDetector{ptrSeen: make(map[interface{}]struct{})}
	return c.Check(reflect.ValueOf(v))
}

type Dict map[interface{}]interface{}

func (d Dict) Set(key interface{}, value interface{}) (string, error) {
	if key == nil {
		return "", errors.New("key cannot be nil")
	}
	d[key] = value
	if isContainer(value) {
		if err := detectCyclicValue(d); err != nil {
			return "", template.UncatchableError(err)
		}
	}
	return "", nil
}

func (d Dict) Get(key interface{}) interface{} {
	out, ok := d[key]
	if !ok {
		switch key.(type) {
		case int:
			out = d[ToInt64(key)]
		case int64:
			out = d[tmplToInt(key)]
		}
	}
	return out
}

func (d Dict) Del(key interface{}) string {
	delete(d, key)
	return ""
}

func (d Dict) HasKey(k interface{}) (ok bool) {
	_, ok = d[k]
	return
}

func (d Dict) MarshalJSON() ([]byte, error) {
	md := make(map[string]interface{})

	for k, v := range d {
		if k == nil {
			return nil, errors.New("key is nil, cannot parse to json")
		}
		krv := reflect.ValueOf(k)
		switch krv.Kind() {
		case reflect.String:
			md[krv.String()] = v
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			md[strconv.FormatInt(krv.Int(), 10)] = v
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			md[strconv.FormatUint(krv.Uint(), 10)] = v
		default:
			return nil, fmt.Errorf("cannot encode dict with key type %s; only string and integer keys are supported", krv.Type())
		}
	}
	return json.Marshal(md)
}

type SDict map[string]interface{}

func (d SDict) Set(key string, value interface{}) (string, error) {
	d[key] = value
	if isContainer(value) {
		if err := detectCyclicValue(d); err != nil {
			return "", template.UncatchableError(err)
		}
	}
	return "", nil
}

func (d SDict) Get(key string) interface{} {
	return d[key]
}

func (d SDict) Del(key string) string {
	delete(d, key)
	return ""
}

func (d SDict) HasKey(k string) (ok bool) {
	_, ok = d[k]
	return
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
	if isContainer(item) {
		if err := detectCyclicValue(s); err != nil {
			return "", template.UncatchableError(err)
		}
	}
	return "", nil
}

func (s Slice) AppendSlice(slice interface{}) (interface{}, error) {
	val, _ := indirect(reflect.ValueOf(slice))
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

type ComponentBuilder struct {
	Components []string
	Values     []interface{}
}

func (s *ComponentBuilder) Add(key string, value interface{}) (interface{}, error) {
	if len(s.Components)+1 > 10000 {
		return nil, errors.New("resulting slice exceeds slice size limit")
	}

	s.Components = append(s.Components, key)
	s.Values = append(s.Values, value)
	return "", nil
}

func (s *ComponentBuilder) AddSlice(key string, slice interface{}) (interface{}, error) {
	val, _ := indirect(reflect.ValueOf(slice))
	switch val.Kind() {
	case reflect.Slice, reflect.Array:
	// this is valid

	default:
		return nil, errors.New("value passed is not an array or slice")
	}

	if len(s.Components)+val.Len() > 10000 {
		return nil, errors.New("resulting slice exceeds slice size limit")
	}

	result := reflect.ValueOf(&s.Values).Elem()
	for i := 0; i < val.Len(); i++ {
		s.Components = append(s.Components, key)
		switch v := val.Index(i).Interface().(type) {
		case nil:
			result = reflect.Append(result, reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem()))

		default:
			result = reflect.Append(result, reflect.ValueOf(v))
		}
	}
	s.Values = result.Interface().([]interface{})

	return "", nil
}

func (s *ComponentBuilder) Merge(toMerge *ComponentBuilder) (interface{}, error) {
	if len(s.Components)+len(toMerge.Components) > 10000 {
		return nil, errors.New("resulting slice exceeds slice size limit")
	}

	for i, k := range toMerge.Components {
		s.Add(k, toMerge.Values[i])
	}

	return "", nil
}

func (s *ComponentBuilder) Get(key string) (result []interface{}) {
	for i, k := range s.Components {
		if k == key {
			result = append(result, s.Values[i])
		}
	}
	return
}

func withOutputLimit(f func(...interface{}) string, limit int) func(...interface{}) (string, error) {
	return func(args ...interface{}) (string, error) {
		out := f(args...)
		if len(out) > limit {
			return "", fmt.Errorf("string grew too long: length %d (max %d)", len(out), limit)
		}
		return out, nil
	}
}

func withOutputLimitF(f func(string, ...interface{}) string, limit int) func(string, ...interface{}) (string, error) {
	return func(format string, args ...interface{}) (string, error) {
		out := f(format, args...)
		if len(out) > limit {
			return "", fmt.Errorf("string grew too long: length %d (max %d)", len(out), limit)
		}
		return out, nil
	}
}
