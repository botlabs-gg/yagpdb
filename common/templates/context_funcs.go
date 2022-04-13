package templates

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/bot"
	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/common/scheduledevents2"
	"github.com/jonas747/discordgo/v2"
	"github.com/jonas747/dstate/v4"
)

var ErrTooManyCalls = errors.New("too many calls to this function")
var ErrTooManyAPICalls = errors.New("too many potential discord api calls function")

func (c *Context) tmplSendDM(s ...interface{}) string {
	if len(s) < 1 || c.IncreaseCheckCallCounter("send_dm", 1) || c.IncreaseCheckGenericAPICall() || c.MS == nil || c.IsExecedByLeaveMessage {
		return ""
	}

	gIcon := discordgo.EndpointGuildIcon(c.GS.ID, c.GS.Icon)

	info := fmt.Sprintf("Custom Command DM from the server **%s**", c.GS.Name)
	embedInfo := fmt.Sprintf("Custom Command DM from the server %s", c.GS.Name)
	msgSend := &discordgo.MessageSend{
		AllowedMentions: discordgo.AllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		},
	}

	switch t := s[0].(type) {
	case *discordgo.MessageEmbed:
		t.Footer = &discordgo.MessageEmbedFooter{
			Text:    embedInfo,
			IconURL: gIcon,
		}
		msgSend.Embed = t
	case *discordgo.MessageSend:
		msgSend = t
		if msgSend.Embed != nil {
			msgSend.Embed.Footer = &discordgo.MessageEmbedFooter{
				Text:    embedInfo,
				IconURL: gIcon,
			}
			break
		}
		if (strings.TrimSpace(msgSend.Content) == "") && (msgSend.File == nil) {
			return ""
		}
		msgSend.Content = info + "\n" + msgSend.Content
	default:
		msgSend.Content = fmt.Sprintf("%s\n%s", info, fmt.Sprint(s...))
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
				// Channel name, look for it
				for _, v := range c.GS.Channels {
					if strings.EqualFold(t, v.Name) && v.Type == discordgo.ChannelTypeGuildText {
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
	if c.IsExecedByLeaveMessage {
		return "", errors.New("Can't use sendTemplateDM on leave msg")
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

			cs = c.GS.GetChannel(cID)
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
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
	}

	return func(channel interface{}, msg interface{}) interface{} {
		if c.IncreaseCheckGenericAPICall() {
			return ""
		}

		cid := c.ChannelArg(channel)
		if cid == 0 {
			return ""
		}

		isDM := cid != c.ChannelArgNoDM(channel)
		gName := c.GS.Name
		info := fmt.Sprintf("Custom Command DM from the server **%s**", gName)
		embedInfo := fmt.Sprintf("Custom Command DM from the server %s", gName)
		icon := discordgo.EndpointGuildIcon(c.GS.ID, c.GS.Icon)

		var m *discordgo.Message
		msgSend := &discordgo.MessageSend{
			AllowedMentions: discordgo.AllowedMentions{
				Parse: parseMentions,
			},
		}
		var err error

		switch typedMsg := msg.(type) {
		case *discordgo.MessageEmbed:
			if isDM {
				typedMsg.Footer = &discordgo.MessageEmbedFooter{
					Text:    embedInfo,
					IconURL: icon,
				}
			}
			msgSend.Embed = typedMsg
		case *discordgo.MessageSend:
			msgSend = typedMsg
			msgSend.AllowedMentions = discordgo.AllowedMentions{Parse: parseMentions}

			if isDM {
				if typedMsg.Embed != nil {
					typedMsg.Embed.Footer = &discordgo.MessageEmbedFooter{
						Text:    embedInfo,
						IconURL: icon,
					}
				} else {
					typedMsg.Content = info + "\n" + typedMsg.Content
				}
			}
		default:
			if isDM {
				msgSend.Content = info + "\n" + ToString(msg)
			} else {
				msgSend.Content = ToString(msg)
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
			ID:      mID,
			Channel: cid,
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEdit.Embed = typedMsg
		case *discordgo.MessageEdit:
			//If both Embed and string are explicitly set as null, give an error message.
			if typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" && typedMsg.Embed != nil && typedMsg.Embed.GetMarshalNil() {
				return "", errors.New("both content and embed cannot be null")
			}
			msgEdit.Content = typedMsg.Content
			msgEdit.Embed = typedMsg.Embed
		default:
			temp := fmt.Sprint(msg)
			msgEdit.Content = &temp
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

func (c *Context) tmplMentionEveryone() string {
	c.CurrentFrame.MentionEveryone = true
	return "@everyone"
}

func (c *Context) tmplMentionHere() string {
	c.CurrentFrame.MentionHere = true
	return "@here"
}

func targetUserID(input interface{}) int64 {
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
		targetID = targetUserID(target)
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

func (c *Context) findRoleByID(id int64) *discordgo.Role {
	for _, r := range c.GS.Roles {
		if r.ID == id {
			return &r
		}
	}

	return nil
}

func (c *Context) findRoleByName(name string) *discordgo.Role {
	for _, r := range c.GS.Roles {
		if strings.EqualFold(r.Name, name) {
			return &r
		}
	}

	return nil
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

//Deletes reactions from a message either via reaction trigger or argument-set of emojis,
//needs channelID, messageID, userID, list of emojis - up to twenty
//can be run once per CC.
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
			uID = targetUserID(args[2].Interface())
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
	return message, nil
}

func (c *Context) tmplGetMember(target interface{}) (*discordgo.Member, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	mID := targetUserID(target)
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
		return nil, nil //dont send an error , a nil output would indicate invalid/unknown channel
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
		return nil, nil //dont send an error , a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetThread(cID)

	if cstate == nil {
		return nil, errors.New("thread not in state")
	}

	return CtxChannelFromCS(cstate), nil
}

func (c *Context) tmplGetChannelOrThread(channel interface{}) (*CtxChannel, error) {

	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil //dont send an error , a nil output would indicate invalid/unknown channel
	}

	cstate := c.GS.GetChannelOrThread(cID)

	if cstate == nil {
		return nil, errors.New("thread/channel not in state")
	}

	return CtxChannelFromCS(cstate), nil
}

func (c *Context) tmplGetChannelPinCount(channel interface{}) (int, error) {
	if c.IncreaseCheckCallCounterPremium("count_pins", 2, 4) {
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

	return len(msg), nil
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

	cID := c.ChannelArgNoDMNoThread(channel)
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

func (c *Context) tmplSort(input interface{}, sortargs ...interface{}) (interface{}, error) {
	if c.IncreaseCheckCallCounterPremium("sortfuncs", 1, 3) {
		return "", ErrTooManyCalls
	}

	inputSlice, _ := indirect(reflect.ValueOf(input))
	switch inputSlice.Kind() {
	case reflect.Slice, reflect.Array:
		// valid
	default:
		return "", fmt.Errorf("Can not use type %s as input to the sort func", inputSlice.Type().String())
	}

	var dict SDict
	var err error

	// We have optional args to set the output of the func
	//
	// Reverse
	// Reverses the order
	// From [0 1 2] to [2 1 0]
	//
	// Subslices
	// By default the function returns a single slice with all the values sorted.
	// Setting subslices to true will make the function return a set of sublices
	// based on the input type/kind
	// From [1 2 3 a b c] to [[1 2 3] [a b c]]
	//
	// Emptyslices
	// By default the function only returns the slices that had an input to them.
	// If you sort only strings, the output would be a slice of strings.
	// But with this flag the function returns all possible slices, this is helpful for indexing
	// From [[1 2 3] [a b c] [map[a:1 b:2]]] to [[1 2 3] [] [a b c] [] [] [map[a:1 b:2]] []]
	//
	// We can have up to 7 subslices total:
	// intSlice, floatSlice, stringSlice, timeSlice, sliceSlice, mapSlice and defaultSlice
	//
	// Note that the output will always be an `Slice` even if all the items
	// of the slice are of a single type/kind
	switch len(sortargs) {
	case 0:
		dict = SDict{
			"reverse":     false,
			"subslices":   false,
			"emptyslices": false,
		}
	case 1:
		dict, err = StringKeyDictionary(sortargs[0])
		if err != nil {
			return "", err
		}
	default:
		dict, err = StringKeyDictionary(sortargs...)
		if err != nil {
			return "", err
		}
	}

	var intSlice, floatSlice, stringSlice, timeSlice, csliceSlice, mapSlice, defaultSlice, outputSlice Slice

	for i := 0; i < inputSlice.Len(); i++ {
		iv, _ := indirect(inputSlice.Index(i))
		switch t := iv.Interface().(type) {
		case int, int64:
			intSlice = append(intSlice, t)
		case *int:
			if t != nil {
				intSlice = append(intSlice, *t)
			}
		case *int64:
			if t != nil {
				intSlice = append(intSlice, *t)
			}
		case float64:
			floatSlice = append(floatSlice, t)
		case *float64:
			if t != nil {
				floatSlice = append(floatSlice, *t)
			}
		case string:
			stringSlice = append(stringSlice, t)
		case *string:
			if t != nil {
				stringSlice = append(stringSlice, *t)
			}
		case time.Time:
			timeSlice = append(timeSlice, t)
		case *time.Time:
			if t != nil {
				timeSlice = append(timeSlice, *t)
			}
		default:
			v := reflect.ValueOf(t)
			switch v.Kind() {
			case reflect.Slice:
				csliceSlice = append(csliceSlice, t)
			case reflect.Map:
				mapSlice = append(mapSlice, t)
			default:
				defaultSlice = append(defaultSlice, t)
			}
		}
	}

	if dict.Get(strings.ToLower("reverse")) == true { // User wants the output in reversed order
		sort.Slice(intSlice, func(i, j int) bool { return ToInt64(intSlice[i]) > ToInt64(intSlice[j]) })
		sort.Slice(floatSlice, func(i, j int) bool { return ToFloat64(floatSlice[i]) > ToFloat64(floatSlice[j]) })
		sort.Slice(stringSlice, func(i, j int) bool { return ToString(stringSlice[i]) > ToString(stringSlice[j]) })
		sort.Slice(timeSlice, func(i, j int) bool { return timeSlice[i].(time.Time).Before(timeSlice[j].(time.Time)) })
		sort.Slice(csliceSlice, func(i, j int) bool { return getLen(csliceSlice[i]) > getLen(csliceSlice[j]) })
		sort.Slice(mapSlice, func(i, j int) bool { return getLen(mapSlice[i]) > getLen(mapSlice[j]) })
	} else { // User wants the output in standard order
		sort.Slice(intSlice, func(i, j int) bool { return ToInt64(intSlice[i]) < ToInt64(intSlice[j]) })
		sort.Slice(floatSlice, func(i, j int) bool { return ToFloat64(floatSlice[i]) < ToFloat64(floatSlice[j]) })
		sort.Slice(stringSlice, func(i, j int) bool { return ToString(stringSlice[i]) < ToString(stringSlice[j]) })
		sort.Slice(timeSlice, func(i, j int) bool { return timeSlice[j].(time.Time).Before(timeSlice[i].(time.Time)) })
		sort.Slice(csliceSlice, func(i, j int) bool { return getLen(csliceSlice[i]) < getLen(csliceSlice[j]) })
		sort.Slice(mapSlice, func(i, j int) bool { return getLen(mapSlice[i]) < getLen(mapSlice[j]) })
	}

	if dict.Get(strings.ToLower("subslices")) == true { // User wants the output to be separated by type/kind
		if dict.Get(strings.ToLower("emptyslices")) == true { // User wants the output to be filled with empty slices
			outputSlice = append(outputSlice, intSlice, floatSlice, stringSlice, timeSlice, csliceSlice, mapSlice, defaultSlice)
		} else { // User only wants the subset of slices that contain data
			if len(intSlice) > 0 {
				outputSlice = append(outputSlice, intSlice)
			}

			if len(floatSlice) > 0 {
				outputSlice = append(outputSlice, floatSlice)
			}

			if len(stringSlice) > 0 {
				outputSlice = append(outputSlice, stringSlice)
			}

			if len(timeSlice) > 0 {
				outputSlice = append(outputSlice, timeSlice)
			}

			if len(csliceSlice) > 0 {
				outputSlice = append(outputSlice, csliceSlice)
			}

			if len(mapSlice) > 0 {
				outputSlice = append(outputSlice, mapSlice)
			}

			if len(defaultSlice) > 0 {
				outputSlice = append(outputSlice, defaultSlice)
			}
		}
	} else { // User wants a single slice output, without any subset
		outputSlice = append(outputSlice, intSlice...)
		outputSlice = append(outputSlice, floatSlice...)
		outputSlice = append(outputSlice, stringSlice...)
		outputSlice = append(outputSlice, timeSlice...)
		outputSlice = append(outputSlice, csliceSlice...)
		outputSlice = append(outputSlice, mapSlice...)
		outputSlice = append(outputSlice, defaultSlice...)
	}

	return outputSlice, nil
}

func getLen(from interface{}) int {
	v, _ := indirect(reflect.ValueOf(from))
	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len()
	default:
		return 0
	}
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

		if strings.HasPrefix(t, "<@&") && strings.HasSuffix(t, ">") && (len(t) > 4) {
			parsedMention, err := strconv.ParseInt(t[3:len(t)-1], 10, 64)
			if err == nil {
				return c.GS.GetRole(parsedMention)
			}
		}

		if t == "@everyone" { // If it's the everyone role, we just use the guild ID
			return c.GS.GetRole(c.GS.ID)
		}

		// It's a name after all
		return c.findRoleByName(t)
	default:
		int64Role := ToInt64(t)
		if int64Role == 0 {
			return nil
		}

		return c.GS.GetRole(int64Role)
	}
}

func (c *Context) getRole(r interface{}) (*discordgo.Role, error) {
	if c.IncreaseCheckStateLock() {
		return nil, ErrTooManyCalls
	}

	return c.FindRole(r), nil
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

func (c *Context) mentionRole(roleInput interface{}) (string, error) {
	if c.IncreaseCheckStateLock() {
		return "", ErrTooManyCalls
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return "", errors.New("role not found")
	}

	if common.ContainsInt64Slice(c.CurrentFrame.MentionRoles, role.ID) {
		return role.Mention(), nil
	}

	c.CurrentFrame.MentionRoles = append(c.CurrentFrame.MentionRoles, role.ID)
	return role.Mention(), nil
}

func (c *Context) tmplMentionRole(roleInput interface{}) (string, error) {
	return c.mentionRole(roleInput)
}

func (c *Context) tmplMentionRoleID(roleID interface{}) (string, error) {
	return c.mentionRole(roleID)
}

func (c *Context) tmplMentionRoleName(roleName string) (string, error) {
	return c.mentionRole(roleName)
}

func (c *Context) hasRole(roleInput interface{}) (bool, error) {
	if c.IncreaseCheckStateLock() {
		return false, ErrTooManyCalls
	}

	if c.MS == nil || c.MS.Member == nil {
		return false, errors.New("Member is nil")
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return false, fmt.Errorf("Role %v not found", roleInput)
	}

	return common.ContainsInt64Slice(c.MS.Member.Roles, role.ID), nil
}

func (c *Context) tmplHasRole(roleInput interface{}) (bool, error) {
	return c.hasRole(roleInput)
}

func (c *Context) tmplHasRoleID(roleID interface{}) (bool, error) {
	return c.hasRole(roleID)
}

func (c *Context) tmplHasRoleName(roleName string) (bool, error) {
	return c.hasRole(roleName)
}

func (c *Context) targetHasRole(target interface{}, roleInput interface{}) (bool, error) {
	if c.IncreaseCheckStateLock() {
		return false, ErrTooManyCalls
	}

	if c.IncreaseCheckGenericAPICall() {
		return false, ErrTooManyAPICalls
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return false, fmt.Errorf("Target %v not found", target)
	}

	ms, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return false, err
	}

	if ms == nil {
		return false, errors.New("MemberState not found")
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return false, fmt.Errorf("Role %v not found", roleInput)
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

func (c *Context) giveRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckStateLock() {
		return "", ErrTooManyCalls
	}

	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = c.validateDurationDelay(optionalArgs[0])
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return "", fmt.Errorf("Target %v not found", target)
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return "", fmt.Errorf("Role %v not found", roleInput)
	}

	if delay > time.Second {
		err := scheduledevents2.ScheduleAddRole(context.Background(), c.GS.ID, targetID, role.ID, time.Now().Add(delay))
		if err != nil {
			return "", err
		}
	} else {
		ms, err := bot.GetMember(c.GS.ID, targetID)
		var hasRole bool
		if ms != nil && err == nil {
			hasRole = common.ContainsInt64Slice(ms.Member.Roles, role.ID)
		}

		if hasRole {
			// User already has this role, nothing to be done
			return "", nil
		}

		err = common.BotSession.GuildMemberRoleAdd(c.GS.ID, targetID, role.ID)
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func (c *Context) tmplGiveRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	return c.giveRole(target, roleInput, optionalArgs...)
}

func (c *Context) tmplGiveRoleID(target interface{}, roleID interface{}, optionalArgs ...interface{}) (string, error) {
	return c.giveRole(target, roleID, optionalArgs...)
}

func (c *Context) tmplGiveRoleName(target interface{}, roleName string, optionalArgs ...interface{}) (string, error) {
	return c.giveRole(target, roleName, optionalArgs...)
}

func (c *Context) addRole(roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckStateLock() {
		return "", ErrTooManyCalls
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
		return "", fmt.Errorf("Role %v not found", roleInput)
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

func (c *Context) takeRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckStateLock() {
		return "", ErrTooManyCalls
	}

	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = c.validateDurationDelay(optionalArgs[0])
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return "", fmt.Errorf("Target %v not found", target)
	}

	role := c.FindRole(roleInput)
	if role == nil {
		return "", fmt.Errorf("Role %v not found", roleInput)
	}

	if delay > time.Second {
		err := scheduledevents2.ScheduleRemoveRole(context.Background(), c.GS.ID, targetID, role.ID, time.Now().Add(delay))
		if err != nil {
			return "", err
		}
	} else {
		ms, err := bot.GetMember(c.GS.ID, targetID)
		hasRole := true
		if ms != nil && err == nil {
			hasRole = common.ContainsInt64Slice(ms.Member.Roles, role.ID)
		}

		if !hasRole {
			// User does not have the role, nothing to be done
			return "", nil
		}

		err = common.BotSession.GuildMemberRoleRemove(c.GS.ID, targetID, role.ID)
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func (c *Context) tmplTakeRole(target interface{}, roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	return c.takeRole(target, roleInput, optionalArgs...)
}

func (c *Context) tmplTakeRoleID(target interface{}, roleID interface{}, optionalArgs ...interface{}) (string, error) {
	return c.takeRole(target, roleID, optionalArgs...)
}

func (c *Context) tmplTakeRoleName(target interface{}, roleName string, optionalArgs ...interface{}) (string, error) {
	return c.takeRole(target, roleName, optionalArgs...)
}

func (c *Context) removeRole(roleInput interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckStateLock() {
		return "", ErrTooManyCalls
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
		return "", fmt.Errorf("Role %v not found", roleInput)
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
