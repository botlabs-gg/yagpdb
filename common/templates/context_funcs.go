package templates

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var (
	ErrTooManyCalls    = errors.New("too many calls to this function")
	ErrTooManyAPICalls = errors.New("too many potential discord api calls function")
)

func (c *Context) tmplSendDM(s ...interface{}) string {
	if len(s) < 1 || c.IncreaseCheckCallCounter("send_dm", 1) || c.IncreaseCheckGenericAPICall() || c.MS == nil || c.ExecutedFrom == ExecutedFromLeave {
		return ""
	}

	msgSend := &discordgo.MessageSend{
		AllowedMentions: discordgo.AllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		},
	}

	switch t := s[0].(type) {
	case *discordgo.MessageEmbed:
		msgSend.Embeds = []*discordgo.MessageEmbed{t}
	case []*discordgo.MessageEmbed:
		msgSend.Embeds = t
	case *discordgo.MessageSend:
		msgSend = t
		if (len(msgSend.Embeds) == 0 && strings.TrimSpace(msgSend.Content) == "") && (msgSend.File == nil) {
			return ""
		}
	default:
		msgSend.Content = fmt.Sprint(s...)
	}
	msgSend.Components = []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Show Server Info",
					Style:    discordgo.PrimaryButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "📬"},
					CustomID: fmt.Sprintf("DM_%d", c.GS.ID),
				},
			},
		},
	}

	channel, err := common.BotSession.UserChannelCreate(c.MS.User.ID)
	if err != nil {
		return ""
	}
	_, _ = common.BotSession.ChannelMessageSendComplex(channel.ID, msgSend)
	return ""
}

func (c *Context) baseChannelArg(v interface{}) *dstate.ChannelState {
	// Look for the channel
	if v == nil && c.CurrentFrame.CS != nil {
		// No channel passed, assume current channel
		return c.CurrentFrame.CS
	}

	var cid int64
	if v != nil {
		switch t := v.(type) {
		case int, int64:
			// Channel id passed
			cid = ToInt64(t)
		case string:
			parsed, err := strconv.ParseInt(t, 10, 64)
			if err == nil {
				// Channel id passed in string format
				cid = parsed
			} else {
				// Channel name, look for it in the all the channels that support text
				for _, v := range c.GS.Channels {
					if strings.EqualFold(t, v.Name) && (v.Type == discordgo.ChannelTypeGuildText || v.Type == discordgo.ChannelTypeGuildVoice || v.Type == discordgo.ChannelTypeGuildForum || v.Type == discordgo.ChannelTypeGuildNews) {
						return &v
					}
				}
				// Do the same for thread names
				for _, v := range c.GS.Threads {
					if strings.EqualFold(t, v.Name) && (v.Type == discordgo.ChannelTypeGuildPublicThread || v.Type == discordgo.ChannelTypeGuildPrivateThread || v.Type == discordgo.ChannelTypeGuildNewsThread) {
						return &v
					}
				}
			}
		}
	}

	return c.GS.GetChannelOrThread(cid)
}

// ChannelArg converts a variety of types of argument into a channel, verifying that it exists
func (c *Context) ChannelArg(v interface{}) int64 {
	cs := c.baseChannelArg(v)
	if cs == nil {
		return 0
	}

	return cs.ID
}

// ChannelArgNoDM is the same as ChannelArg but will not accept DM channels
func (c *Context) ChannelArgNoDM(v interface{}) int64 {
	cs := c.baseChannelArg(v)
	if cs == nil || cs.IsPrivate() {
		return 0
	}

	return cs.ID
}

func (c *Context) ChannelArgNoDMNoThread(v interface{}) int64 {
	cs := c.baseChannelArg(v)
	if cs == nil || cs.IsPrivate() || cs.Type.IsThread() {
		return 0
	}

	return cs.ID
}

func (c *Context) tmplSendTemplateDM(name string, data ...interface{}) (interface{}, error) {
	if c.ExecutedFrom == ExecutedFromLeave {
		return "", errors.New("can't use sendTemplateDM on leave msg")
	}

	return c.sendNestedTemplate(nil, true, name, data...)
}

func (c *Context) tmplSendTemplate(channel interface{}, name string, data ...interface{}) (interface{}, error) {
	return c.sendNestedTemplate(channel, false, name, data...)
}

func (c *Context) sendNestedTemplate(channel interface{}, dm bool, name string, data ...interface{}) (interface{}, error) {
	if c.IncreaseCheckCallCounter("exec_child", 3) {
		return "", ErrTooManyCalls
	}
	if name == "" {
		return "", errors.New("no template name passed")
	}
	if c.CurrentFrame.isNestedTemplate {
		return "", errors.New("can't call this in a nested template")
	}

	t := c.CurrentFrame.parsedTemplate.Lookup(name)
	if t == nil {
		return "", errors.New("unknown template")
	}

	var cs *dstate.ChannelState
	// find the new context channel
	if !dm {
		if channel == nil {
			cs = c.CurrentFrame.CS
		} else {
			cID := c.ChannelArg(channel)
			if cID == 0 {
				return "", errors.New("unknown channel")
			}

			cs = c.GS.GetChannelOrThread(cID)
			if cs == nil {
				return "", errors.New("unknown channel")
			}
		}
	} else {
		if c.CurrentFrame.SendResponseInDM {
			cs = c.CurrentFrame.CS
		} else {
			ch, err := common.BotSession.UserChannelCreate(c.MS.User.ID)
			if err != nil {
				return "", err
			}

			cs = &dstate.ChannelState{
				GuildID: c.GS.ID,
				ID:      ch.ID,
				Name:    c.MS.User.Username,
				Type:    discordgo.ChannelTypeDM,
			}
		}
	}

	oldFrame := c.newContextFrame(cs)
	defer func() {
		c.CurrentFrame = oldFrame
	}()

	if dm {
		c.CurrentFrame.SendResponseInDM = oldFrame.SendResponseInDM
	} else if channel == nil {
		// inherit
		c.CurrentFrame.SendResponseInDM = oldFrame.SendResponseInDM
	}

	// pass some data
	if len(data) > 1 {
		dict, _ := Dictionary(data...)
		c.Data["TemplateArgs"] = dict
		if !c.checkSafeDictNoRecursion(dict, 0) {
			return nil, errors.New("trying to pass the entire current context data in as templateargs, this is not needed, just use nil and access all other data normally")
		}
	} else if len(data) == 1 {
		if cast, ok := data[0].(map[string]interface{}); ok && reflect.DeepEqual(cast, c.Data) {
			return nil, errors.New("trying to pass the entire current context data in as templateargs, this is not needed, just use nil and access all other data normally")
		}
		c.Data["TemplateArgs"] = data[0]
	}

	// and finally execute the child template
	c.CurrentFrame.parsedTemplate = t
	resp, err := c.executeParsed()
	if err != nil {
		return "", err
	}

	m, err := c.SendResponse(resp)
	if err != nil {
		return "", err
	}

	if m != nil {
		return m.ID, err
	}
	return "", err
}

func (c *Context) checkSafeStringDictNoRecursion(d SDict, n int) bool {
	if n > 1000 {
		return false
	}

	for _, v := range d {
		if cast, ok := v.(Dict); ok {
			if !c.checkSafeDictNoRecursion(cast, n+1) {
				return false
			}
		}

		if cast, ok := v.(*Dict); ok {
			if !c.checkSafeDictNoRecursion(*cast, n+1) {
				return false
			}
		}

		if cast, ok := v.(SDict); ok {
			if !c.checkSafeStringDictNoRecursion(cast, n+1) {
				return false
			}
		}

		if cast, ok := v.(*SDict); ok {
			if !c.checkSafeStringDictNoRecursion(*cast, n+1) {
				return false
			}
		}

		if reflect.DeepEqual(v, c.Data) {
			return false
		}
	}

	return true
}

func (c *Context) checkSafeDictNoRecursion(d Dict, n int) bool {
	if n > 1000 {
		return false
	}

	for _, v := range d {
		if cast, ok := v.(Dict); ok {
			if !c.checkSafeDictNoRecursion(cast, n+1) {
				return false
			}
		}

		if cast, ok := v.(*Dict); ok {
			if !c.checkSafeDictNoRecursion(*cast, n+1) {
				return false
			}
		}

		if cast, ok := v.(SDict); ok {
			if !c.checkSafeStringDictNoRecursion(cast, n+1) {
				return false
			}
		}

		if cast, ok := v.(*SDict); ok {
			if !c.checkSafeStringDictNoRecursion(*cast, n+1) {
				return false
			}
		}

		if reflect.DeepEqual(v, c.Data) {
			return false
		}
	}

	return true
}

func (c *Context) tmplSendMessage(filterSpecialMentions bool, returnID bool) func(channel interface{}, msg interface{}) interface{} {
	var repliedUser bool
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
		repliedUser = true
	}

	return func(channel interface{}, msg interface{}) interface{} {
		if c.IncreaseCheckGenericAPICall() {
			return ""
		}

		sendType := sendMessageGuildChannel
		cid := c.ChannelArg(channel)
		if cid == 0 {
			return ""
		}

		if cid != c.ChannelArgNoDM(channel) {
			sendType = sendMessageDM
		}

		var m *discordgo.Message
		msgSend := &discordgo.MessageSend{
			AllowedMentions: discordgo.AllowedMentions{
				Parse:       parseMentions,
				RepliedUser: repliedUser,
			},
		}
		var err error

		switch typedMsg := msg.(type) {
		case *discordgo.MessageEmbed:
			msgSend.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgSend.Embeds = typedMsg
		case *discordgo.MessageSend:
			msgSend = typedMsg
			if !filterSpecialMentions {
				msgSend.AllowedMentions = discordgo.AllowedMentions{Parse: parseMentions, RepliedUser: repliedUser}
			}
			if msgSend.Reference != nil && msgSend.Reference.ChannelID == 0 {
				msgSend.Reference.ChannelID = cid
			}
		default:
			msgSend.Content = ToString(msg)
		}

		if sendType == sendMessageDM {
			msgSend.Components = []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
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

		m, err = common.BotSession.ChannelMessageSendComplex(cid, msgSend)

		if err == nil && returnID {
			return m.ID
		}

		return ""
	}
}

func (c *Context) tmplEditMessage(filterSpecialMentions bool) func(channel interface{}, msgID interface{}, msg interface{}) (interface{}, error) {
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
	}
	return func(channel interface{}, msgID interface{}, msg interface{}) (interface{}, error) {
		if c.IncreaseCheckGenericAPICall() {
			return "", ErrTooManyAPICalls
		}

		cid := c.ChannelArgNoDM(channel)
		if cid == 0 {
			return "", errors.New("unknown channel")
		}

		mID := ToInt64(msgID)
		msgEdit := &discordgo.MessageEdit{
			ID:              mID,
			Channel:         cid,
			AllowedMentions: discordgo.AllowedMentions{Parse: parseMentions},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEdit.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgEdit.Embeds = typedMsg
		case *discordgo.MessageEdit:
			embeds := make([]*discordgo.MessageEmbed, 0, len(typedMsg.Embeds))
			// If there are no Embeds and string are explicitly set as null, give an error message.
			if typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" {
				if len(typedMsg.Embeds) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}

				// only keep valid embeds
				for _, e := range typedMsg.Embeds {
					if e != nil && !e.GetMarshalNil() {
						embeds = append(typedMsg.Embeds, e)
					}
				}
				if len(embeds) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}
			}
			msgEdit.Content = typedMsg.Content
			msgEdit.Embeds = typedMsg.Embeds
			msgEdit.Components = typedMsg.Components
			msgEdit.AllowedMentions = typedMsg.AllowedMentions
		default:
			temp := fmt.Sprint(msg)
			msgEdit.Content = &temp
		}

		if !filterSpecialMentions {
			msgEdit.AllowedMentions = discordgo.AllowedMentions{
				Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone},
			}
		}

		_, err = common.BotSession.ChannelMessageEditComplex(msgEdit)
		if err != nil {
			return "", err
		}

		return "", nil
	}
}

func (c *Context) tmplPinMessage(unpin bool) func(channel, msgID interface{}) (string, error) {
	return func(channel, msgID interface{}) (string, error) {
		if c.IncreaseCheckCallCounter("message_pins", 5) {
			return "", ErrTooManyCalls
		}

		cID := c.ChannelArgNoDM(channel)
		if cID == 0 {
			return "", errors.New("unknown channel")
		}
		mID := ToInt64(msgID)
		var err error
		if unpin {
			err = common.BotSession.ChannelMessageUnpin(cID, mID)
		} else {
			err = common.BotSession.ChannelMessagePin(cID, mID)
		}
		return "", err
	}
}

func (c *Context) tmplPublishMessage(channel, msgID interface{}) (string, error) {
	// Too heavily ratelimited by Discord to allow rapid feeds to publish
	if c.ExecutedFrom == ExecutedFromLeave || c.ExecutedFrom == ExecutedFromJoin {
		return "", errors.New("cannot publish messages from a join/leave feed")
	}

	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckCallCounter("message_publish", 1) {
		return "", ErrTooManyCalls
	}

	cID := c.ChannelArgNoDM(channel)
	if cID == 0 {
		return "", errors.New("unknown channel")
	}
	mID := ToInt64(msgID)

	// Don't crosspost if the message has already been crossposted
	msg, err := common.BotSession.ChannelMessage(cID, mID)
	if err != nil {
		return "", errors.New("message not found")
	}
	messageAlreadyCrossposted := msg.Flags&discordgo.MessageFlagsCrossPosted == discordgo.MessageFlagsCrossPosted
	if messageAlreadyCrossposted {
		return "", nil
	}

	_, err = common.BotSession.ChannelMessageCrosspost(cID, mID)
	return "", err
}

func (c *Context) tmplPublishResponse() (string, error) {
	// Too heavily ratelimited by Discord to allow rapid feeds to publish
	if c.ExecutedFrom == ExecutedFromLeave || c.ExecutedFrom == ExecutedFromJoin {
		return "", errors.New("cannot publish messages from a join/leave feed")
	}

	if c.CurrentFrame.CS.Type == discordgo.ChannelTypeGuildNews {
		c.CurrentFrame.PublishResponse = true
	}
	return "", nil
}

func (c *Context) tmplMentionEveryone() string {
	c.CurrentFrame.MentionEveryone = true
	return "@everyone"
}

func (c *Context) tmplMentionHere() string {
	c.CurrentFrame.MentionHere = true
	return "@here"
}

func TargetUserID(input interface{}) int64 {
	switch t := input.(type) {
	case *discordgo.User:
		return t.ID
	case string:
		str := strings.TrimSpace(t)
		if strings.HasPrefix(str, "<@") && strings.HasSuffix(str, ">") && (len(str) > 4) {
			trimmed := str[2 : len(str)-1]
			if trimmed[0] == '!' {
				trimmed = trimmed[1:]
			}
			str = trimmed
		}

		return ToInt64(str)
	default:
		return ToInt64(input)
	}
}

const DiscordRoleLimit = 250

func (c *Context) tmplSetRoles(target interface{}, input interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	var targetID int64
	if target == nil {
		// nil denotes the context member
		if c.MS != nil {
			targetID = c.MS.User.ID
		}
	} else {
		targetID = TargetUserID(target)
	}

	if targetID == 0 {
		return "", nil
	}

	if c.IncreaseCheckCallCounter("set_roles"+discordgo.StrID(targetID), 1) {
		return "", errors.New("too many calls for specific user ID (max 1 / user)")
	}

	rv, _ := indirect(reflect.ValueOf(input))
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		// ok
	default:
		return "", errors.New("value passed was not an array or slice")
	}

	// use a map to easily handle duplicate roles
	roles := make(map[int64]struct{})

	// if users supply a slice of roles that does not contain a managed role of the member, the Discord API returns an error.
	// add in the managed roles of the member by default so the user doesn't have to do it manually every time.
	ms, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return "", nil
	}

	for _, id := range ms.Member.Roles {
		r := c.GS.GetRole(id)
		if r != nil && r.Managed {
			roles[id] = struct{}{}
		}
	}

	for i := 0; i < rv.Len(); i++ {
		v, _ := indirect(rv.Index(i))
		switch v.Kind() {
		case reflect.Int, reflect.Int64:
			roles[v.Int()] = struct{}{}
		case reflect.String:
			id, err := strconv.ParseInt(v.String(), 10, 64)
			if err != nil {
				return "", errors.New("could not parse string value into role ID")
			}
			roles[id] = struct{}{}
		case reflect.Struct:
			if r, ok := v.Interface().(discordgo.Role); ok {
				roles[r.ID] = struct{}{}
				break
			}
			fallthrough
		default:
			return "", errors.New("could not parse value into role ID")
		}

		if len(roles) > DiscordRoleLimit {
			return "", fmt.Errorf("more than %d unique roles passed; %[1]d is the Discord role limit", DiscordRoleLimit)
		}
	}

	// convert map to slice of keys (role IDs)
	rs := make([]string, 0, len(roles))
	for id := range roles {
		rs = append(rs, discordgo.StrID(id))
	}

	err = common.BotSession.GuildMemberEdit(c.GS.ID, targetID, rs)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (c *Context) findRoleByName(name string) *discordgo.Role {
	for _, r := range c.GS.Roles {
		if strings.EqualFold(r.Name, name) {
			return &r
		}
	}

	return nil
}

func (c *Context) tmplHasPermissions(needed int64) (bool, error) {
	if c.IncreaseCheckGenericAPICall() {
		return false, ErrTooManyAPICalls
	}

	if c.MS == nil {
		return false, nil
	}

	if needed < 0 {
		return false, nil
	}

	if needed == 0 {
		return true, nil
	}

	return c.hasPerms(c.MS, c.CurrentFrame.CS.ID, needed)
}

func (c *Context) tmplTargetHasPermissions(target interface{}, needed int64) (bool, error) {
	if c.IncreaseCheckGenericAPICall() {
		return false, ErrTooManyAPICalls
	}

	targetID := TargetUserID(target)
	if targetID == 0 {
		return false, nil
	}

	if needed < 0 {
		return false, nil
	}

	if needed == 0 {
		return true, nil
	}

	ms, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return false, err
	}

	return c.hasPerms(ms, c.CurrentFrame.CS.ID, needed)
}

func (c *Context) tmplGetTargetPermissionsIn(target interface{}, channel interface{}) (int64, error) {
	if c.IncreaseCheckGenericAPICall() {
		return 0, ErrTooManyAPICalls
	}

	targetID := TargetUserID(target)
	if targetID == 0 {
		return 0, nil
	}

	channelID := c.ChannelArgNoDM(channel)
	if channelID == 0 {
		return 0, nil
	}

	ms, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return 0, err
	}

	return c.GS.GetMemberPermissions(channelID, ms.User.ID, ms.Member.Roles)
}

func (c *Context) hasPerms(ms *dstate.MemberState, channelID int64, needed int64) (bool, error) {
	perms, err := c.GS.GetMemberPermissions(channelID, ms.User.ID, ms.Member.Roles)
	if err != nil {
		return false, err
	}

	if perms&needed == needed {
		return true, nil
	}

	if perms&discordgo.PermissionAdministrator != 0 {
		return true, nil
	}

	return false, nil
}

func (c *Context) tmplDelResponse(args ...interface{}) string {
	dur := 10
	if len(args) > 0 {
		dur = int(ToInt64(args[0]))
	}
	if dur > 86400 {
		dur = 86400
	}

	c.CurrentFrame.DelResponseDelay = dur
	c.CurrentFrame.DelResponse = true
	return ""
}

func (c *Context) tmplDelTrigger(args ...interface{}) string {
	if c.Msg != nil {
		return c.tmplDelMessage(c.Msg.ChannelID, c.Msg.ID, args...)
	}

	return ""
}

func (c *Context) tmplDelMessage(channel, msgID interface{}, args ...interface{}) string {
	cID := c.ChannelArgNoDM(channel)
	if cID == 0 {
		return ""
	}

	mID := ToInt64(msgID)

	dur := 10
	if len(args) > 0 {
		dur = int(ToInt64(args[0]))
	}

	if dur > 86400 {
		dur = 86400
	}

	MaybeScheduledDeleteMessage(c.GS.ID, cID, mID, dur)

	return ""
}

// Deletes reactions from a message either via reaction trigger or argument-set of emojis,
// needs channelID, messageID, userID, list of emojis - up to twenty
// can be run once per CC.
func (c *Context) tmplDelMessageReaction(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 4 {
			return reflect.Value{}, errors.New("not enough arguments (need channelID, messageID, userID, emoji)")
		}

		var cArg interface{}
		if args[0].IsValid() {
			cArg = args[0].Interface()
		}

		cID := c.ChannelArg(cArg)
		if cID == 0 {
			return reflect.ValueOf("non-existing channel"), nil
		}

		var mID, uID int64

		if args[1].IsValid() {
			mID = ToInt64(args[1].Interface())
		}

		if args[2].IsValid() {
			uID = TargetUserID(args[2].Interface())
		}

		if uID == 0 {
			return reflect.ValueOf("non-existing user"), nil
		}

		for _, reaction := range args[3:] {

			if c.IncreaseCheckCallCounter("del_reaction_message", 10) {
				return reflect.Value{}, ErrTooManyCalls
			}

			if err := common.BotSession.MessageReactionRemove(cID, mID, reaction.String(), uID); err != nil {
				return reflect.Value{}, err
			}
		}
		return reflect.ValueOf(""), nil
	}

	return callVariadic(f, false, values...)
}

func (c *Context) tmplDelAllMessageReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 2 {
			return reflect.Value{}, errors.New("not enough arguments (need channelID, messageID, emojis[optional])")
		}

		var cArg interface{}
		if args[0].IsValid() {
			cArg = args[0].Interface()
		}

		cID := c.ChannelArg(cArg)
		if cID == 0 {
			return reflect.ValueOf("non-existing channel"), nil
		}

		var mID int64
		if args[1].IsValid() {
			mID = ToInt64(args[1].Interface())
		}

		if len(args) > 2 {
			for _, emoji := range args[2:] {
				if c.IncreaseCheckCallCounter("del_reaction_message", 10) {
					return reflect.Value{}, ErrTooManyCalls
				}

				if err := common.BotSession.MessageReactionRemoveEmoji(cID, mID, emoji.String()); err != nil {
					return reflect.Value{}, err
				}
			}
			return reflect.ValueOf(""), nil
		}

		if c.IncreaseCheckGenericAPICall() {
			return reflect.Value{}, ErrTooManyAPICalls
		}
		common.BotSession.MessageReactionsRemoveAll(cID, mID)
		return reflect.ValueOf(""), nil
	}

	return callVariadic(f, false, values...)
}

func (c *Context) tmplGetMessage(channel, msgID interface{}) (*discordgo.Message, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	cID := c.ChannelArgNoDM(channel)
	if cID == 0 {
		return nil, nil
	}

	mID := ToInt64(msgID)

	message, _ := common.BotSession.ChannelMessage(cID, mID)
	if message != nil {
		// get message endpoint doesn't return guild ID, so just patch it in to make message.Link work
		message.GuildID = c.GS.ID
	}
	return message, nil
}

func (c *Context) tmplGetMember(target interface{}) (*discordgo.Member, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	mID := TargetUserID(target)
	if mID == 0 {
		return nil, nil
	}

	member, _ := bot.GetMember(c.GS.ID, mID)
	if member == nil {
		return nil, nil
	}

	return member.DgoMember(), nil
}

func (c *Context) tmplGetChannel(channel interface{}) (*CtxChannel, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil // dont send an error , a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetChannel(cID)

	if cstate == nil {
		return nil, errors.New("channel not in state")
	}

	return CtxChannelFromCS(cstate), nil
}

func (c *Context) tmplGetThread(channel interface{}) (*CtxChannel, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil // dont send an error , a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetThread(cID)

	if cstate == nil {
		return nil, errors.New("thread not in state")
	}

	return CtxChannelFromCS(cstate), nil
}

func (c *Context) AddThreadToGuildSet(t *dstate.ChannelState) {
	// Perform a copy so we don't mutate global array
	gsCopy := *c.GS
	gsCopy.Threads = make([]dstate.ChannelState, len(c.GS.Threads), len(c.GS.Threads)+1)
	copy(gsCopy.Threads, c.GS.Threads)

	// Add new thread to copied guild state
	gsCopy.Threads = append(gsCopy.Threads, *t)
	c.GS = &gsCopy
}

func (c *Context) tmplCreateThread(channel, msgID, name interface{}, optionals ...interface{}) (*CtxChannel, error) {
	if c.IncreaseCheckCallCounterPremium("create_thread", 1, 1) {
		return nil, ErrTooManyCalls
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil // dont send an error, a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetChannel(cID)
	if cstate == nil {
		return nil, errors.New("channel not in state")
	}

	start := &discordgo.ThreadStart{
		Name:                ToString(name),
		Type:                discordgo.ChannelTypeGuildPublicThread,
		AutoArchiveDuration: 10080, // 7 days
		Invitable:           false,
	}
	mID := ToInt64(msgID)
	for index, opt := range optionals {
		switch index {
		case 0:
			switch opt := opt.(type) {
			case bool:
				if opt {
					start.Type = discordgo.ChannelTypeGuildPrivateThread
				}
			default:
				return nil, errors.New("createThread 'private' must be a boolean")
			}
		case 1:
			duration := tmplToInt(opt)
			if duration < 60 || duration > 10080 {
				return nil, errors.New("createThread 'auto_archive_duration' must be and integer between 60 and 10080")
			}
		case 2:
			switch opt := opt.(type) {
			case bool:
				if opt {
					start.Invitable = true
				}
			default:
				return nil, errors.New("createThread 'invitable' must be a boolean")
			}
		default:
			return nil, errors.New("createThread: Too many arguments")
		}
	}

	var ctxThread *discordgo.Channel
	var err error
	if mID > 0 {
		ctxThread, err = common.BotSession.MessageThreadStartComplex(cID, mID, start)
	} else {
		ctxThread, err = common.BotSession.ThreadStartComplex(cID, start)
	}

	if err != nil {
		return nil, nil // dont send an error, a nil output would indicate invalid/unknown channel
	}

	tstate := dstate.ChannelStateFromDgo(ctxThread)
	c.AddThreadToGuildSet(&tstate)

	return CtxChannelFromCS(&tstate), nil
}

// This function can delete both basic threads and forum threads
func (c *Context) tmplDeleteThread(thread interface{}) (string, error) {
	if c.IncreaseCheckCallCounterPremium("delete_thread", 1, 1) {
		return "", ErrTooManyCalls
	}

	cID := c.ChannelArg(thread)
	if cID == 0 {
		return "", nil // dont send an error, a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetThread(cID)
	if cstate == nil {
		return "", nil // dont send an error, a nil output would indicate invalid/unknown channel
	}

	common.BotSession.ChannelDelete(cID)
	return "", nil
}

func (c *Context) tmplThreadMemberAdd(threadID, memberID interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	tID := c.ChannelArg(threadID)
	if tID == 0 {
		return ""
	}

	cstate := c.GS.GetThread(tID)
	if cstate == nil {
		return ""
	}

	targetID := TargetUserID(memberID)
	if targetID == 0 {
		return ""
	}

	common.BotSession.ThreadMemberAdd(tID, discordgo.StrID(targetID))
	return ""
}

func (c *Context) tmplThreadMemberRemove(threadID, memberID interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	tID := c.ChannelArg(threadID)
	if tID == 0 {
		return ""
	}

	cstate := c.GS.GetThread(tID)
	if cstate == nil {
		return ""
	}

	targetID := TargetUserID(memberID)
	if targetID == 0 {
		return ""
	}

	common.BotSession.ThreadMemberRemove(tID, discordgo.StrID(targetID))
	return ""
}

func ConvertTagNameToId(c *dstate.ChannelState, tagName string) int64 {
	if c.AvailableTags == nil {
		return 0
	}

	// walk available tags list and see if there's a match
	for _, tag := range c.AvailableTags {
		if tag.Name == tagName {
			return tag.ID
		}
	}

	return 0
}

func ProcessOptionalForumPostArgs(c *dstate.ChannelState, values ...interface{}) (int, []int64, error) {
	if len(values) == 0 {
		return c.DefaultThreadRateLimitPerUser, nil, nil
	}

	threadSdict, err := StringKeyDictionary(values...)
	if err != nil {
		return 0, nil, err
	}

	rateLimit := c.DefaultThreadRateLimitPerUser
	var tags []int64 = nil
	for key, val := range threadSdict {

		key = strings.ToLower(key)
		switch key {
		case "slowmode":
			rateLimit = tmplToInt(val)
		case "tags":
			if c.AvailableTags == nil {
				break
			}

			v, _ := indirect(reflect.ValueOf(val))
			const maxTags = 5 // discord limit
			if v.Kind() == reflect.String {
				tag := ConvertTagNameToId(c, ToString(val))
				// ensure supplied id is valid
				if tag > 0 {
					tags = []int64{tag}
				}
			} else if v.Kind() == reflect.Slice {
				// used to get rid of any duplicate tags the user might have sent
				seen := make(map[string]struct{})
				size := v.Len()
				if size > maxTags {
					size = maxTags
				}

				tags = make([]int64, 0, size)
				for i := 0; i < v.Len() && len(seen) < size; i++ {
					name := ToString(v.Index(i).Interface())
					if len(name) == 0 {
						continue
					}

					_, ok := seen[name]
					if ok {
						continue
					}

					// try to convert and check if the id is valid
					tag := ConvertTagNameToId(c, name)
					if tag == 0 {
						continue
					}

					seen[name] = struct{}{}
					tags = append(tags, tag)
				}

			} else {
				return 0, nil, errors.New("tags must be of type string or cslice")
			}
		default:
			return 0, nil, errors.New(`invalid key "` + key + `"`)
		}
	}

	return rateLimit, tags, nil
}

func (c *Context) tmplCreateForumPost(channel, name, content interface{}, optional ...interface{}) (*CtxChannel, error) {
	// shares same counter as create thread
	if c.IncreaseCheckCallCounterPremium("create_thread", 1, 1) {
		return nil, ErrTooManyCalls
	}

	if content == nil {
		return nil, errors.New("post content must not be nil")
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil // dont send an error, a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetChannel(cID)
	if cstate == nil {
		return nil, errors.New("channel not in state")
	}

	if !cstate.Type.IsForum() {
		return nil, errors.New("must specify a forum channel")
	}

	rateLimit, tags, err := ProcessOptionalForumPostArgs(cstate, optional...)
	if err != nil {
		return nil, err
	}

	start := &discordgo.ThreadStart{
		Name:             ToString(name),
		Type:             discordgo.ChannelTypeGuildPublicThread,
		Invitable:        false,
		RateLimitPerUser: rateLimit,
		AppliedTags:      tags,
	}

	var msgData *discordgo.MessageSend
	switch v := content.(type) {
	case string:
		if len(v) == 0 {
			return nil, errors.New("post content must be non-zero length")
		}
		msgData, _ = CreateMessageSend("content", v)
	case *discordgo.MessageEmbed:
		msgData, _ = CreateMessageSend("embed", v)
	case *discordgo.MessageSend:
		msgData = v
	default:
		return nil, errors.New("post content must be string, embed, or complex message")
	}

	thread, err := common.BotSession.ForumThreadStartComplex(cID, start, msgData)
	if err != nil {
		return nil, errors.New("unable to create forum post")
	}

	tstate := dstate.ChannelStateFromDgo(thread)
	tstate.AppliedTags = tags
	c.AddThreadToGuildSet(&tstate)

	return CtxChannelFromCS(&tstate), nil
}

func (c *Context) tmplGetChannelOrThread(channel interface{}) (*CtxChannel, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil // dont send an error , a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetChannelOrThread(cID)

	if cstate == nil {
		return nil, errors.New("thread/channel not in state")
	}

	return CtxChannelFromCS(cstate), nil
}

func (c *Context) tmplGetChannelPins(pinCount bool) func(channel interface{}) (interface{}, error) {
	return func(channel interface{}) (interface{}, error) {
		if c.IncreaseCheckCallCounterPremium("channel_pins", 2, 4) {
			return 0, ErrTooManyCalls
		}

		cID := c.ChannelArgNoDM(channel)
		if cID == 0 {
			return 0, errors.New("unknown channel")
		}

		msg, err := common.BotSession.ChannelMessagesPinned(cID)
		if err != nil {
			return 0, err
		}

		if pinCount {
			return len(msg), nil
		}

		pinnedMessages := make([]discordgo.Message, 0, len(msg))
		for _, m := range msg {
			pinnedMessages = append(pinnedMessages, *m)
		}

		return pinnedMessages, nil
	}
}

func (c *Context) tmplAddReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if c.Msg == nil {
			return reflect.Value{}, nil
		}

		for _, reaction := range args {
			if c.IncreaseCheckCallCounter("add_reaction_trigger", 20) {
				return reflect.Value{}, ErrTooManyCalls
			}

			if err := common.BotSession.MessageReactionAdd(c.Msg.ChannelID, c.Msg.ID, reaction.String()); err != nil {
				return reflect.Value{}, err
			}
		}
		return reflect.ValueOf(""), nil
	}

	return callVariadic(f, true, values...)
}

func (c *Context) tmplAddResponseReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		for _, reaction := range args {
			if c.IncreaseCheckCallCounter("add_reaction_response", 20) {
				return reflect.Value{}, ErrTooManyCalls
			}

			c.CurrentFrame.AddResponseReactionNames = append(c.CurrentFrame.AddResponseReactionNames, reaction.String())
		}
		return reflect.ValueOf(""), nil
	}

	return callVariadic(f, true, values...)
}

func (c *Context) tmplAddMessageReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 2 {
			return reflect.Value{}, errors.New("not enough arguments (need channel and message-id)")
		}

		// cArg := args[0].Interface()
		var cArg interface{}
		if args[0].IsValid() {
			cArg = args[0].Interface()
		}

		cID := c.ChannelArg(cArg)
		if cID == 0 {
			return reflect.ValueOf(""), nil
		}

		var mID int64
		if args[1].IsValid() {
			mID = ToInt64(args[1].Interface())
		}

		for i, reaction := range args {
			if i < 2 {
				continue
			}

			if c.IncreaseCheckCallCounter("add_reaction_message", 20) {
				return reflect.Value{}, ErrTooManyCalls
			}

			if err := common.BotSession.MessageReactionAdd(cID, mID, reaction.String()); err != nil {
				return reflect.Value{}, err
			}
		}
		return reflect.ValueOf(""), nil
	}

	return callVariadic(f, false, values...)
}

func (c *Context) tmplCurrentUserAgeHuman() string {
	t := bot.SnowflakeToTime(c.MS.User.ID)

	humanized := common.HumanizeDuration(common.DurationPrecisionHours, time.Since(t))
	if humanized == "" {
		humanized = "Less than an hour"
	}

	return humanized
}

func (c *Context) tmplCurrentUserAgeMinutes() int {
	t := bot.SnowflakeToTime(c.MS.User.ID)
	d := time.Since(t)

	return int(d.Seconds() / 60)
}

func (c *Context) tmplCurrentUserCreated() time.Time {
	t := bot.SnowflakeToTime(c.MS.User.ID)
	return t
}

func (c *Context) tmplSleep(duration interface{}) (string, error) {
	seconds := tmplToInt(duration)
	if c.secondsSlept+seconds > 60 || seconds < 1 {
		return "", errors.New("can sleep for max 60 seconds combined")
	}

	c.secondsSlept += seconds
	time.Sleep(time.Duration(seconds) * time.Second)
	return "", nil
}

func (c *Context) compileRegex(r string) (*regexp.Regexp, error) {
	if c.RegexCache == nil {
		c.RegexCache = make(map[string]*regexp.Regexp)
	}

	cached, ok := c.RegexCache[r]
	if ok {
		return cached, nil
	}

	if len(c.RegexCache) >= 10 {
		return nil, ErrTooManyAPICalls
	}

	compiled, err := regexp.Compile(r)
	if err != nil {
		return nil, err
	}

	c.RegexCache[r] = compiled

	return compiled, nil
}

func (c *Context) reFind(r, s string) (string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return "", err
	}

	return compiled.FindString(s), nil
}

func (c *Context) reFindAll(r, s string, i ...int) ([]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	var n int
	if len(i) > 0 {
		n = i[0]
	}

	if n > 1000 || n <= 0 {
		n = 1000
	}

	return compiled.FindAllString(s, n), nil
}

func (c *Context) reFindAllSubmatches(r, s string, i ...int) ([][]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	var n int
	if len(i) > 0 {
		n = i[0]
	}

	if n > 100 || n <= 0 {
		n = 100
	}

	return compiled.FindAllStringSubmatch(s, n), nil
}

func (c *Context) reReplace(r, s, repl string) (string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return "", err
	}

	return compiled.ReplaceAllString(s, repl), nil
}

func (c *Context) reSplit(r, s string, i ...int) ([]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	var n int
	if len(i) > 0 {
		n = i[0]
	}

	if n > 500 || n <= 0 {
		n = 500
	}

	return compiled.Split(s, n), nil
}

func (c *Context) tmplEditChannelName(channel interface{}, newName string) (string, error) {
	if c.IncreaseCheckCallCounter("edit_channel", 10) {
		return "", ErrTooManyCalls
	}

	cID := c.ChannelArgNoDM(channel)
	if cID == 0 {
		return "", errors.New("unknown channel")
	}

	if c.IncreaseCheckCallCounter("edit_channel_"+strconv.FormatInt(cID, 10), 2) {
		return "", ErrTooManyCalls
	}

	_, err := common.BotSession.ChannelEdit(cID, newName)
	return "", err
}

func (c *Context) tmplEditChannelTopic(channel interface{}, newTopic string) (string, error) {
	if c.IncreaseCheckCallCounter("edit_channel", 10) {
		return "", ErrTooManyCalls
	}

	cID := c.ChannelArgNoDMNoThread(channel)
	if cID == 0 {
		return "", errors.New("unknown channel")
	}

	if c.IncreaseCheckCallCounter("edit_channel_"+strconv.FormatInt(cID, 10), 2) {
		return "", ErrTooManyCalls
	}

	edit := &discordgo.ChannelEdit{
		Topic: newTopic,
	}

	_, err := common.BotSession.ChannelEditComplex(cID, edit)
	return "", err
}

func (c *Context) tmplOnlineCount() (int, error) {
	if c.IncreaseCheckCallCounter("online_users", 1) {
		return 0, ErrTooManyCalls
	}

	gwc, err := common.BotSession.GuildWithCounts(c.GS.ID)
	if err != nil {
		return 0, err
	}

	return gwc.ApproximatePresenceCount, nil
}

// DEPRECATED: this function will likely not return
func (c *Context) tmplOnlineCountBots() (int, error) {
	// if c.IncreaseCheckCallCounter("online_bots", 1) {
	// 	return 0, ErrTooManyCalls
	// }

	// botCount := 0

	// for _, v := range c.GS.Members {
	// 	if v.Bot && v.PresenceSet && v.PresenceStatus != dstate.StatusOffline {
	// 		botCount++
	// 	}
	// }

	return 0, nil
}

func (c *Context) tmplEditNickname(Nickname string) (string, error) {
	if c.IncreaseCheckCallCounter("edit_nick", 2) {
		return "", ErrTooManyCalls
	}

	if c.MS == nil {
		return "", nil
	}

	if strings.Compare(c.MS.Member.Nick, Nickname) == 0 {
		return "", nil
	}

	err := common.BotSession.GuildMemberNickname(c.GS.ID, c.MS.User.ID, Nickname)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (c *Context) tmplSort(input interface{}, args ...interface{}) (interface{}, error) {
	if c.IncreaseCheckCallCounterPremium("sort", 1, 3) {
		return "", ErrTooManyCalls
	}

	v, _ := indirect(reflect.ValueOf(input))
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		// ok
	default:
		return "", fmt.Errorf("cannot sort value of type %T", input)
	}

	opts, err := parseSortOpts(args...)
	if err != nil {
		return nil, err
	}

	type cmpVal struct {
		Key  reflect.Value // Key is the value to sort by.
		Orig reflect.Value
	}
	vals := make([]cmpVal, v.Len())
	cmp := invalidComparator
	for i := 0; i < v.Len(); i++ {
		el, _ := indirect(v.Index(i))
		key := el
		if opts.Key.IsValid() {
			key, err = indexContainer(el, opts.Key)
			if err != nil {
				return nil, err
			}
			key, _ = indirect(key)
		}

		curCmp, err := comparatorOf(key)
		if err != nil {
			return nil, err
		}

		if i == 0 {
			cmp = curCmp
		} else if curCmp != cmp {
			return nil, errors.New("input contains incompatible element types")
		}
		vals[i] = cmpVal{key, el}
	}

	sort.SliceStable(vals, func(i, j int) bool {
		if opts.Reverse {
			i, j = j, i
		}
		return cmp.Less(vals[i].Key, vals[j].Key)
	})
	out := make(Slice, len(vals))
	for i, v := range vals {
		out[i] = v.Orig.Interface()
	}
	return out, nil
}

var defaultSortOpts = sortOpts{Reverse: false, Key: reflect.Value{}}

type sortOpts struct {
	Reverse bool
	Key     reflect.Value
}

func parseSortOpts(args ...interface{}) (*sortOpts, error) {
	opts := defaultSortOpts
	if len(args) == 0 {
		return &opts, nil
	}

	dict, err := StringKeyDictionary(args...)
	if err != nil {
		return nil, err
	}

	for k, v := range dict {
		switch {
		case strings.EqualFold(k, "reverse"):
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("expected reverse option to be of type bool, but got type %T instead", v)
			}
			opts.Reverse = b
		case strings.EqualFold(k, "key"):
			opts.Key = reflect.ValueOf(v)
		default:
			return nil, fmt.Errorf("invalid option %q", k)
		}
	}
	return &opts, nil
}

func indexContainer(container, key reflect.Value) (reflect.Value, error) {
	container, _ = indirect(container)
	key, _ = indirect(key)

	switch container.Kind() {
	case reflect.Array, reflect.Slice:
		switch key.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := int(key.Int())
			if i < 0 || i >= container.Len() {
				return reflect.Value{}, fmt.Errorf("index %d out of range", i)
			}
			return container.Index(i), nil

		case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u := key.Uint()
			if u >= uint64(container.Len()) {
				return reflect.Value{}, fmt.Errorf("index %d out of range", u)
			}
			return container.Index(int(u)), nil

		default:
			return reflect.Value{}, fmt.Errorf("cannot index array/slice by key of type %T", key.Type())
		}

	case reflect.Map:
		if key.Type().AssignableTo(container.Type().Key()) {
			v := container.MapIndex(key)
			if !v.IsValid() {
				return reflect.Value{}, fmt.Errorf("key %v not found in map", key)
			}
			return v, nil
		}

	case reflect.Struct:
		if key.Kind() != reflect.String {
			return reflect.Value{}, fmt.Errorf("cannot index struct with non-string key")
		}

		s := key.String()
		ft, ok := container.Type().FieldByName(s)
		if !ok {
			return reflect.Value{}, fmt.Errorf("no field named %q in %s struct", s, container.Type())
		}
		if !ft.IsExported() {
			return reflect.Value{}, fmt.Errorf("field %q of %s struct is not exported", s, container.Type())
		}
		return container.FieldByName(s), nil
	}

	return reflect.Value{}, fmt.Errorf("cannot index value of type %s", container.Type())
}

type comparator int

const (
	invalidComparator comparator = iota
	intComparator
	uintComparator
	floatComparator
	stringComparator
	timeComparator
)

func (c comparator) Less(a, b reflect.Value) bool {
	switch c {
	case intComparator:
		return a.Int() < b.Int()
	case uintComparator:
		return a.Uint() < b.Uint()
	case floatComparator:
		af, bf := a.Float(), b.Float()
		return af < bf || (math.IsNaN(af) && !math.IsNaN(bf))
	case stringComparator:
		return a.String() < b.String()
	case timeComparator:
		return a.Interface().(time.Time).Before(b.Interface().(time.Time))
	default:
		panic("invalid comparator")
	}
}

var timeType = reflect.TypeOf(time.Time{})

func comparatorOf(v reflect.Value) (comparator, error) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intComparator, nil
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uintComparator, nil
	case reflect.Float32, reflect.Float64:
		return floatComparator, nil
	case reflect.String:
		return stringComparator, nil
	default:
		if v.Type() == timeType {
			return timeComparator, nil
		}
		return invalidComparator, fmt.Errorf("cannot compare value of type %s", v.Type())
	}
}

func (c *Context) tmplEditInteractionResponse(filterSpecialMentions bool) func(interactionToken, msgID, msg interface{}) (interface{}, error) {
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
	}
	return func(interactionToken, msgID, msg interface{}) (interface{}, error) {
		if c.IncreaseCheckGenericAPICall() {
			return "", ErrTooManyAPICalls
		}

		_, token := c.tokenArg(interactionToken)
		if token == "" {
			return "", errors.New("invalid interaction token")
		}

		var editOriginal bool
		mID := ToInt64(msgID)
		if mID == 0 {
			editOriginal = true
		}

		msgEdit := &discordgo.WebhookParams{
			AllowedMentions: &discordgo.AllowedMentions{Parse: parseMentions},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEdit.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgEdit.Embeds = typedMsg
		case *discordgo.MessageEdit:
			embeds := make([]*discordgo.MessageEmbed, 0, len(typedMsg.Embeds))
			//If there are no Embeds and string are explicitly set as null, give an error message.
			if typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" {
				if len(typedMsg.Embeds) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}

				//only keep valid embeds
				for _, e := range typedMsg.Embeds {
					if e != nil && !e.GetMarshalNil() {
						embeds = append(typedMsg.Embeds, e)
					}
				}
				if len(embeds) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}
			}
			msgEdit.Content = *typedMsg.Content
			msgEdit.Embeds = typedMsg.Embeds
			msgEdit.Components = typedMsg.Components
			msgEdit.AllowedMentions = &typedMsg.AllowedMentions
		default:
			temp := fmt.Sprint(msg)
			msgEdit.Content = temp
		}

		if !filterSpecialMentions {
			msgEdit.AllowedMentions = &discordgo.AllowedMentions{
				Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone},
			}
		}

		if editOriginal {
			_, err = common.BotSession.EditOriginalInteractionResponse(common.BotApplication.ID, token, msgEdit)
			if err == nil && token == c.CurrentFrame.Interaction.Token {
				c.CurrentFrame.Interaction.RespondedTo = true
			}
		} else {
			_, err = common.BotSession.EditFollowupMessage(common.BotApplication.ID, token, mID, msgEdit)
		}

		if err != nil {
			return "", err
		}

		return "", nil
	}
}

func (c *Context) tmplEphemeralResponse() string {
	if c.CurrentFrame.Interaction != nil {
		c.CurrentFrame.EphemeralResponse = true
	}
	return ""
}

func (c *Context) tmplSendModal(modal interface{}) (interface{}, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckCallCounter("interaction_response", 1) {
		return "", ErrTooManyCalls
	}

	var typedModal *discordgo.InteractionResponse
	switch m := modal.(type) {
	case *discordgo.InteractionResponse:
		typedModal = m
	case discordgo.InteractionResponse:
		typedModal = &m
	default:
		return "", errors.New("invalid modal passed to sendModal")
	}

	if typedModal.Type != discordgo.InteractionResponseModal {
		return "", errors.New("invalid modal passed to sendModal")
	}

	if c.CurrentFrame.Interaction == nil {
		return "", errors.New("no interaction data in context")
	}

	err := common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, c.CurrentFrame.Interaction.Token, typedModal)
	if err != nil {
		return "", err
	}
	c.CurrentFrame.Interaction.RespondedTo = true
	return "", nil
}

func (c *Context) tmplSendInteractionResponse(filterSpecialMentions bool, returnID bool) func(interactionToken interface{}, msg interface{}) interface{} {
	var repliedUser bool
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
		repliedUser = true
	}

	return func(interactionToken interface{}, msg interface{}) interface{} {
		if c.IncreaseCheckGenericAPICall() {
			return ""
		}

		sendType, token := c.tokenArg(interactionToken)
		if token == "" {
			return ""
		}

		var m *discordgo.Message
		msgSend := &discordgo.InteractionResponseData{
			AllowedMentions: &discordgo.AllowedMentions{
				Parse:       parseMentions,
				RepliedUser: repliedUser,
			},
		}
		var err error

		switch typedMsg := msg.(type) {
		case *discordgo.MessageEmbed:
			msgSend.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgSend.Embeds = typedMsg
		case *discordgo.MessageSend:
			msgSend.Content = typedMsg.Content
			msgSend.Embeds = typedMsg.Embeds
			msgSend.Components = typedMsg.Components
			msgSend.Flags = typedMsg.Flags
			msgSend.Files = typedMsg.Files
			if !filterSpecialMentions {
				msgSend.AllowedMentions = &discordgo.AllowedMentions{Parse: parseMentions, RepliedUser: repliedUser}
			}
		default:
			msgSend.Content = ToString(msg)
		}

		switch sendType {
		case sendMessageInteractionResponse:
			if c.IncreaseCheckCallCounter("interaction_response", 1) {
				return ""
			}
			err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, token, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: msgSend,
			})
			if err == nil {
				if token == c.CurrentFrame.Interaction.Token {
					c.CurrentFrame.Interaction.RespondedTo = true
				}
				if returnID {
					m, err = common.BotSession.GetOriginalInteractionResponse(common.BotApplication.ID, token)
				}
			}
		case sendMessageInteractionFollowup:
			m, err = common.BotSession.CreateFollowupMessage(common.BotApplication.ID, token, &discordgo.WebhookParams{
				Content:         msgSend.Content,
				Components:      msgSend.Components,
				Embeds:          msgSend.Embeds,
				AllowedMentions: msgSend.AllowedMentions,
				Flags:           int64(msgSend.Flags),
			})
		}

		if err == nil && returnID {
			return m.ID
		}

		return ""
	}
}

func (c *Context) tmplUpdateMessage(filterSpecialMentions bool) func(msg interface{}) (interface{}, error) {
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
	}
	return func(msg interface{}) (interface{}, error) {
		if c.IncreaseCheckGenericAPICall() {
			return "", ErrTooManyAPICalls
		}

		if c.IncreaseCheckCallCounter("interaction_response", 1) {
			return "", ErrTooManyCalls
		}

		msgEdit := &discordgo.InteractionResponseData{
			AllowedMentions: &discordgo.AllowedMentions{Parse: parseMentions},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEdit.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgEdit.Embeds = typedMsg
		case *discordgo.MessageEdit:
			embeds := make([]*discordgo.MessageEmbed, 0, len(typedMsg.Embeds))
			//If there are no Embeds and string are explicitly set as null, give an error message.
			if typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" {
				if len(typedMsg.Embeds) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}

				//only keep valid embeds
				for _, e := range typedMsg.Embeds {
					if e != nil && !e.GetMarshalNil() {
						embeds = append(typedMsg.Embeds, e)
					}
				}
				if len(embeds) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}
			}
			msgEdit.Content = *typedMsg.Content
			msgEdit.Embeds = typedMsg.Embeds
			msgEdit.Components = typedMsg.Components
			msgEdit.AllowedMentions = &typedMsg.AllowedMentions
		default:
			temp := fmt.Sprint(msg)
			msgEdit.Content = temp
		}

		if !filterSpecialMentions {
			msgEdit.AllowedMentions = &discordgo.AllowedMentions{
				Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone},
			}
		}

		err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, c.CurrentFrame.Interaction.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: msgEdit,
		})

		if err != nil {
			return "", err
		}

		c.CurrentFrame.Interaction.RespondedTo = true
		return "", nil
	}
}

func (c *Context) tokenArg(interactionToken interface{}) (sendType sendMessageType, token string) {
	sendType = sendMessageInteractionFollowup

	token, ok := interactionToken.(string)
	if !ok {
		if interactionToken == nil && c.CurrentFrame.Interaction != nil {
			// no token provided, assume current interaction
			token = c.CurrentFrame.Interaction.Token
		} else {
			return
		}
	}

	if c.CurrentFrame.Interaction != nil && token == c.CurrentFrame.Interaction.Token && !c.CurrentFrame.Interaction.RespondedTo {
		sendType = sendMessageInteractionResponse
	}
	return
}

// c.FindRole accepts all possible role inputs (names, IDs and mentions)
// and tries to find them on the current context
func (c *Context) FindRole(role interface{}) *discordgo.Role {
	switch t := role.(type) {
	case string:
		parsed, err := strconv.ParseInt(t, 10, 64)
		if err == nil {
			return c.GS.GetRole(parsed)
		}

		if len(t) > 4 && strings.HasPrefix(t, "<@&") && strings.HasSuffix(t, ">") {
			parsedMention, err := strconv.ParseInt(t[3:len(t)-1], 10, 64)
			if err == nil {
				return c.GS.GetRole(parsedMention)
			}
		}

		// If it's the everyone role, we just use the guild ID
		if t == "@everyone" {
			return c.GS.GetRole(c.GS.ID)
		}

		// It's a name after all
		return c.findRoleByName(t)
	case *discordgo.Role:
		return t
	case discordgo.Role:
		return &t
	default:
		int64Role := ToInt64(t)
		if int64Role == 0 {
			return nil
		}

		return c.GS.GetRole(int64Role)
	}
}

func (c *Context) getRole(roleInput interface{}) (*discordgo.Role, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyCalls
	}

	return c.FindRole(roleInput), nil
}

func (c *Context) tmplGetRole(roleInput interface{}) (*discordgo.Role, error) {
	return c.getRole(roleInput)
}

func (c *Context) tmplGetRoleID(roleID interface{}) (*discordgo.Role, error) {
	return c.getRole(roleID)
}

func (c *Context) tmplGetRoleName(roleName string) (*discordgo.Role, error) {
	return c.getRole(roleName)
}

func (c *Context) mentionRole(roleInput interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return ""
	}

	if common.ContainsInt64Slice(c.CurrentFrame.MentionRoles, role.ID) {
		return role.Mention()
	}

	c.CurrentFrame.MentionRoles = append(c.CurrentFrame.MentionRoles, role.ID)
	return role.Mention()
}

func (c *Context) tmplMentionRole(roleInput interface{}) string {
	return c.mentionRole(roleInput)
}

func (c *Context) tmplMentionRoleID(roleID interface{}) string {
	return c.mentionRole(roleID)
}

func (c *Context) tmplMentionRoleName(roleName string) string {
	return c.mentionRole(roleName)
}

func (c *Context) hasRole(roleInput interface{}) bool {
	if c.IncreaseCheckGenericAPICall() {
		return false
	}

	if c.MS == nil || c.MS.Member == nil {
		return false
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return false
	}

	return common.ContainsInt64Slice(c.MS.Member.Roles, role.ID)
}

func (c *Context) tmplHasRole(roleInput interface{}) bool {
	return c.hasRole(roleInput)
}

func (c *Context) tmplHasRoleID(roleID interface{}) bool {
	return c.hasRole(roleID)
}

func (c *Context) tmplHasRoleName(roleName string) bool {
	return c.hasRole(roleName)
}

func (c *Context) targetHasRole(target interface{}, roleInput interface{}) (bool, error) {
	if c.IncreaseCheckGenericAPICall() {
		return false, ErrTooManyAPICalls
	}

	targetID := TargetUserID(target)
	if targetID == 0 {
		return false, fmt.Errorf("target %v not found", target)
	}

	ms, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return false, err
	}

	if ms == nil {
		return false, errors.New("member not found in state")
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return false, fmt.Errorf("role %v not found", roleInput)
	}

	return common.ContainsInt64Slice(ms.Member.Roles, role.ID), nil
}

func (c *Context) tmplTargetHasRole(target interface{}, roleInput interface{}) (bool, error) {
	return c.targetHasRole(target, roleInput)
}

func (c *Context) tmplTargetHasRoleID(target interface{}, roleID interface{}) (bool, error) {
	return c.targetHasRole(target, roleID)
}

func (c *Context) tmplTargetHasRoleName(target interface{}, roleName string) (bool, error) {
	return c.targetHasRole(target, roleName)
}

func (c *Context) giveRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = c.validateDurationDelay(optionalArgs[0])
	}

	targetID := TargetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return ""
	}

	if delay > time.Second {
		err := scheduledevents2.ScheduleAddRole(context.Background(), c.GS.ID, targetID, role.ID, time.Now().Add(delay))
		if err != nil {
			return ""
		}
	} else {
		ms, err := bot.GetMember(c.GS.ID, targetID)
		var hasRole bool
		if ms != nil && err == nil {
			hasRole = common.ContainsInt64Slice(ms.Member.Roles, role.ID)
		}

		if hasRole {
			// User already has this role, nothing to be done
			return ""
		}

		err = common.BotSession.GuildMemberRoleAdd(c.GS.ID, targetID, role.ID)
		if err != nil {
			return ""
		}
	}

	return ""
}

func (c *Context) tmplGiveRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) string {
	return c.giveRole(target, roleInput, optionalArgs...)
}

func (c *Context) tmplGiveRoleID(target interface{}, roleID interface{}, optionalArgs ...interface{}) string {
	return c.giveRole(target, roleID, optionalArgs...)
}

func (c *Context) tmplGiveRoleName(target interface{}, roleName string, optionalArgs ...interface{}) string {
	return c.giveRole(target, roleName, optionalArgs...)
}

func (c *Context) addRole(roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = c.validateDurationDelay(optionalArgs[0])
	}

	if c.MS == nil {
		return "", errors.New("tmplAddRole called on context with nil MemberState")
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return "", fmt.Errorf("role %v not found", roleInput)
	}

	if delay > time.Second {
		err := scheduledevents2.ScheduleAddRole(context.Background(), c.GS.ID, c.MS.User.ID, role.ID, time.Now().Add(delay))
		if err != nil {
			return "", err
		}
	} else {
		err := common.AddRoleDS(c.MS, role.ID)
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func (c *Context) tmplAddRole(roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	return c.addRole(roleInput, optionalArgs...)
}

func (c *Context) tmplAddRoleID(roleID interface{}, optionalArgs ...interface{}) (string, error) {
	return c.addRole(roleID, optionalArgs...)
}

func (c *Context) tmplAddRoleName(roleName string, optionalArgs ...interface{}) (string, error) {
	return c.addRole(roleName, optionalArgs...)
}

func (c *Context) takeRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = c.validateDurationDelay(optionalArgs[0])
	}

	targetID := TargetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return ""
	}

	if delay > time.Second {
		err := scheduledevents2.ScheduleRemoveRole(context.Background(), c.GS.ID, targetID, role.ID, time.Now().Add(delay))
		if err != nil {
			return ""
		}
	} else {
		ms, err := bot.GetMember(c.GS.ID, targetID)
		hasRole := true
		if ms != nil && err == nil {
			hasRole = common.ContainsInt64Slice(ms.Member.Roles, role.ID)
		}

		if !hasRole {
			return ""
		}

		err = common.BotSession.GuildMemberRoleRemove(c.GS.ID, targetID, role.ID)
		if err != nil {
			return ""
		}
	}

	return ""
}

func (c *Context) tmplTakeRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) string {
	return c.takeRole(target, roleInput, optionalArgs...)
}

func (c *Context) tmplTakeRoleID(target interface{}, roleID interface{}, optionalArgs ...interface{}) string {
	return c.takeRole(target, roleID, optionalArgs...)
}

func (c *Context) tmplTakeRoleName(target interface{}, roleName string, optionalArgs ...interface{}) string {
	return c.takeRole(target, roleName, optionalArgs...)
}

func (c *Context) removeRole(roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = c.validateDurationDelay(optionalArgs[0])
	}

	if c.MS == nil {
		return "", errors.New("removeRole called on context with nil MemberState")
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return "", fmt.Errorf("role %v not found", roleInput)
	}

	if delay > time.Second {
		err := scheduledevents2.ScheduleRemoveRole(context.Background(), c.GS.ID, c.MS.User.ID, role.ID, time.Now().Add(delay))
		if err != nil {
			return "", err
		}
	} else {
		err := common.RemoveRoleDS(c.MS, role.ID)
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func (c *Context) tmplRemoveRole(roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	return c.removeRole(roleInput, optionalArgs...)
}

func (c *Context) tmplRemoveRoleID(roleID interface{}, optionalArgs ...interface{}) (string, error) {
	return c.removeRole(roleID, optionalArgs...)
}

func (c *Context) tmplRemoveRoleName(roleName string, optionalArgs ...interface{}) (string, error) {
	return c.removeRole(roleName, optionalArgs...)
}

func (c *Context) validateDurationDelay(in interface{}) time.Duration {
	switch t := in.(type) {
	case int, int64:
		return time.Second * ToDuration(t)
	case string:
		conv := ToInt64(t)
		if conv != 0 {
			return time.Second * ToDuration(conv)
		}

		return ToDuration(t)
	default:
		return ToDuration(t)
	}
}
