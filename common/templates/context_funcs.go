package templates

import (
	"context"
	"errors"
	"fmt"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
)

var ErrTooManyCalls = errors.New("Too many calls to this function")
var ErrTooManyAPICalls = errors.New("Too many potential discord api calls function")

func (c *Context) tmplSendDM(s ...interface{}) string {
	if len(s) < 1 || c.IncreaseCheckCallCounter("send_dm", 1) || c.MS == nil {
		return ""
	}

	c.GS.RLock()
	gName := c.GS.Guild.Name
	memberID := c.MS.ID
	c.GS.RUnlock()

	info := fmt.Sprintf("Custom Command DM From the server **%s**", gName)

	// Send embed
	if embed, ok := s[0].(*discordgo.MessageEmbed); ok {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: info,
		}

		bot.SendDMEmbed(memberID, embed)
		return ""
	}

	msg := fmt.Sprint(s...)
	msg = fmt.Sprintf("%s\n%s", info, msg)
	bot.SendDM(memberID, msg)
	return ""
}

func (c *Context) ChannelArg(v interface{}) int64 {

	c.GS.RLock()
	defer c.GS.RUnlock()

	// Look for the channel
	if v == nil && c.CS != nil {
		// No channel passed, assume current channel
		return c.CS.ID
	}

	verifiedExistence := false
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
						cid = v.ID
						verifiedExistence = true
						break
					}
				}
			}
		}
	}

	if !verifiedExistence {
		// Make sure the channel is part of the guild
		for k, _ := range c.GS.Channels {
			if k == cid {
				verifiedExistence = true
				break
			}
		}
	}

	if !verifiedExistence {
		return 0
	}

	return cid
}

func (c *Context) tmplSendMessage(filterSpecialMentions bool, returnID bool) func(channel interface{}, msg interface{}) interface{} {
	return func(channel interface{}, msg interface{}) interface{} {
		if c.IncreaseCheckGenericAPICall() {
			return ""
		}

		cid := c.ChannelArg(channel)
		if cid == 0 {
			return ""
		}

		var m *discordgo.Message
		var err error
		if embed, ok := msg.(*discordgo.MessageEmbed); ok {
			m, err = common.BotSession.ChannelMessageSendEmbed(cid, embed)
		} else {
			strMsg := fmt.Sprint(msg)

			if filterSpecialMentions {
				strMsg = common.EscapeSpecialMentions(strMsg)
			}

			m, err = common.BotSession.ChannelMessageSend(cid, strMsg)
		}

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

		cid := c.ChannelArg(channel)
		if cid == 0 {
			return "", errors.New("Unknown channel")
		}

		mID := ToInt64(msgID)

		var err error
		if embed, ok := msg.(*discordgo.MessageEmbed); ok {
			_, err = common.BotSession.ChannelMessageEditEmbed(cid, mID, embed)
		} else {
			strMsg := fmt.Sprint(msg)

			if filterSpecialMentions {
				strMsg = common.EscapeSpecialMentions(strMsg)
			}

			_, err = common.BotSession.ChannelMessageEdit(cid, mID, strMsg)
		}

		if err != nil {
			return "", err
		}

		return "", nil
	}
}

func (c *Context) tmplMentionEveryone() string {
	c.MentionEveryone = true
	return " @everyone "
}

func (c *Context) tmplMentionHere() string {
	c.MentionHere = true
	return " @here "
}

func (c *Context) tmplMentionRoleID(roleID interface{}) string {
	if c.IncreaseCheckStateLock() {
		return ""
	}

	var role int64
	switch r := roleID.(type) {
	case int64:
		role = r
	case int:
		role = int64(r)
	case string:
		role, _ = strconv.ParseInt(r, 10, 64)
	default:
		return ""
	}

	r := c.GS.RoleCopy(true, role)
	if r == nil {
		return "(role not found)"
	}

	if common.ContainsInt64Slice(c.MentionRoles, role) {
		return "<@&" + discordgo.StrID(role) + ">"
	}

	c.MentionRoles = append(c.MentionRoles, role)
	return " <@&" + discordgo.StrID(role) + "> "
}

func (c *Context) tmplMentionRoleName(role string) string {
	if c.IncreaseCheckStateLock() {
		return ""
	}

	var found *discordgo.Role
	c.GS.RLock()
	for _, r := range c.GS.Guild.Roles {
		if r.Name == role {
			if !common.ContainsInt64Slice(c.MentionRoles, r.ID) {
				c.MentionRoles = append(c.MentionRoles, r.ID)
				found = r
			}
		}
	}
	c.GS.RUnlock()
	if found == nil {
		return "(role not found)"
	}

	return " <@&" + discordgo.StrID(found.ID) + "> "
}

func (c *Context) tmplHasRoleID(roleID interface{}) bool {
	role := ToInt64(roleID)
	if role == 0 {
		return false
	}

	contains := common.ContainsInt64Slice(c.MS.Roles, role)
	return contains
}

func (c *Context) tmplHasRoleName(name string) (bool, error) {
	if c.IncreaseCheckStateLock() {
		return false, ErrTooManyCalls
	}

	c.GS.RLock()
	defer c.GS.RUnlock()

	for _, r := range c.GS.Guild.Roles {
		if strings.EqualFold(r.Name, name) {
			if common.ContainsInt64Slice(c.MS.Roles, r.ID) {
				return true, nil
			}

			return false, nil

		}
	}

	// Role not found, default to false
	return false, nil
}

func targetUserID(input interface{}) int64 {
	switch t := input.(type) {
	case *discordgo.User:
		return t.ID
	default:
		return ToInt64(input)
	}
}

func (c *Context) tmplTargetHasRoleID(target interface{}, roleID interface{}) bool {
	if c.IncreaseCheckStateLock() {
		return false
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return false
	}

	ts, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return false
	}

	role := ToInt64(roleID)
	if role == 0 {
		return false
	}

	contains := common.ContainsInt64Slice(ts.Roles, role)

	return contains

}

func (c *Context) tmplTargetHasRoleName(target interface{}, name string) bool {
	if c.IncreaseCheckStateLock() {
		return false
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return false
	}

	ts, err := bot.GetMember(c.GS.ID, targetID)
	if err != nil {
		return false
	}

	c.GS.RLock()

	for _, r := range c.GS.Guild.Roles {
		if strings.EqualFold(r.Name, name) {
			if common.ContainsInt64Slice(ts.Roles, r.ID) {
				c.GS.RUnlock()
				return true
			}

			c.GS.RUnlock()
			return false
		}
	}

	c.GS.RUnlock()
	return false

}

func (c *Context) tmplGiveRoleID(target interface{}, roleID interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := ToInt64(roleID)
	if role == 0 {
		return ""
	}

	// Check to see if we can save a API request here
	c.GS.RLock()
	ms := c.GS.Member(false, targetID)
	hasRole := false
	if ms != nil {
		hasRole = common.ContainsInt64Slice(ms.Roles, role)
	}
	c.GS.RUnlock()

	if !hasRole {
		common.BotSession.GuildMemberRoleAdd(c.GS.ID, targetID, role)
	}

	return ""
}

func (c *Context) tmplGiveRoleName(target interface{}, name string) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := int64(0)
	c.GS.RLock()
	for _, r := range c.GS.Guild.Roles {
		if strings.EqualFold(r.Name, name) {
			role = r.ID

			// Maybe save a api request
			ms := c.GS.Member(false, targetID)
			hasRole := false
			if ms != nil {
				hasRole = common.ContainsInt64Slice(ms.Roles, role)
			}

			if hasRole {
				c.GS.RUnlock()
				return ""
			}

			break
		}
	}
	c.GS.RUnlock()

	if role == 0 {
		return ""
	}

	common.BotSession.GuildMemberRoleAdd(c.GS.ID, targetID, role)

	return ""
}

func (c *Context) tmplTakeRoleID(target interface{}, roleID interface{}, optionalArgs ...interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	delay := 0
	if len(optionalArgs) > 0 {
		delay = tmplToInt(optionalArgs[0])
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := ToInt64(roleID)
	if role == 0 {
		return ""
	}

	// Check to see if we can save a API request here, if this isn't delayed
	if delay <= 0 {
		c.GS.RLock()
		ms := c.GS.Member(false, targetID)
		hasRole := true
		if ms != nil && ms.MemberSet {
			hasRole = common.ContainsInt64Slice(ms.Roles, role)
		}
		c.GS.RUnlock()

		if !hasRole {
			return ""
		}
	}

	if delay > 0 {
		scheduledevents2.ScheduleRemoveRole(context.Background(), c.GS.ID, targetID, role, time.Now().Add(time.Second*time.Duration(delay)))
	} else {
		common.BotSession.GuildMemberRoleRemove(c.GS.ID, targetID, role)
	}

	return ""
}

func (c *Context) tmplTakeRoleName(target interface{}, name string, optionalArgs ...interface{}) string {
	if c.IncreaseCheckGenericAPICall() {
		return ""
	}

	delay := 0
	if len(optionalArgs) > 0 {
		delay = tmplToInt(optionalArgs[0])
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := int64(0)
	c.GS.RLock()
	for _, r := range c.GS.Guild.Roles {
		if strings.EqualFold(r.Name, name) {
			role = r.ID

			// Maybe save a api request, but only if this is not delayed
			if delay <= 0 {
				ms := c.GS.Member(false, targetID)
				hasRole := true
				if ms != nil && ms.MemberSet {
					hasRole = common.ContainsInt64Slice(ms.Roles, role)
				}

				if !hasRole {
					c.GS.RUnlock()
					return ""
				}
			}

			break
		}
	}
	c.GS.RUnlock()

	if role == 0 {
		return ""
	}

	if delay > 0 {
		scheduledevents2.ScheduleRemoveRole(context.Background(), c.GS.ID, targetID, role, time.Now().Add(time.Second*time.Duration(delay)))
	} else {
		common.BotSession.GuildMemberRoleRemove(c.GS.ID, targetID, role)
	}

	return ""
}

func (c *Context) tmplAddRoleID(role interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.MS == nil {
		return "", nil
	}

	rid := ToInt64(role)
	if rid == 0 {
		return "", errors.New("No role id specified")
	}

	err := common.BotSession.GuildMemberRoleAdd(c.GS.ID, c.MS.ID, rid)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (c *Context) tmplRemoveRoleID(role interface{}, optionalArgs ...interface{}) (string, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	delay := 0
	if len(optionalArgs) > 0 {
		delay = tmplToInt(optionalArgs[0])
	}

	if c.MS == nil {
		return "", nil
	}

	rid := ToInt64(role)
	if rid == 0 {
		return "", errors.New("No role id specified")
	}

	if delay > 0 {
		scheduledevents2.ScheduleRemoveRole(context.Background(), c.GS.ID, c.MS.ID, rid, time.Now().Add(time.Second*time.Duration(delay)))
	} else {
		common.BotSession.GuildMemberRoleRemove(c.GS.ID, c.MS.ID, rid)
	}

	return "", nil
}

func (c *Context) tmplDelResponse(args ...interface{}) string {
	dur := 10
	if len(args) > 0 {
		dur = int(ToInt64(args[0]))
	}
	if dur > 86400 {
		dur = 86400
	}

	c.DelResponseDelay = dur
	c.DelResponse = true
	return ""
}

func (c *Context) tmplDelTrigger(args ...interface{}) string {
	if c.Msg != nil {
		return c.tmplDelMessage(c.Msg.ChannelID, c.Msg.ID, args...)
	}

	return ""
}

func (c *Context) tmplDelMessage(channel, msgID interface{}, args ...interface{}) string {
	cID := c.ChannelArg(channel)
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

func (c *Context) tmplGetMessage(channel, msgID interface{}) (*discordgo.Message, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	cID := c.ChannelArg(channel)
	if cID == 0 {
		return nil, nil
	}

	mID := ToInt64(msgID)

	message, _ := common.BotSession.ChannelMessage(cID, mID)
	return message, nil
}

func (c *Context) tmplGetMember(id interface{}) (*discordgo.Member, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	mID := ToInt64(id)

	member, _ := bot.GetMember(c.GS.ID, mID)
	if member == nil {
		return nil, nil
	}

	return member.DGoCopy(), nil
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

			c.AddResponseReactionNames = append(c.AddResponseReactionNames, reaction.String())
		}
		return reflect.ValueOf(""), nil
	}

	return callVariadic(f, true, values...)
}

func (c *Context) tmplAddMessageReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 2 {
			return reflect.Value{}, errors.New("Not enough arguments (need channel and message-id)")
		}

		// cArg := args[0].Interface()
		var cArg interface{}
		if args[0].IsValid() {
			cArg = args[0].Interface()
		}

		cID := c.ChannelArg(cArg)
		mID := ToInt64(args[1].Interface())

		if cID == 0 {
			return reflect.ValueOf(""), nil
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
	t := bot.SnowflakeToTime(c.MS.ID)

	humanized := common.HumanizeDuration(common.DurationPrecisionHours, time.Since(t))
	if humanized == "" {
		humanized = "Less than an hour"
	}

	return humanized
}

func (c *Context) tmplCurrentUserAgeMinutes() int {
	t := bot.SnowflakeToTime(c.MS.ID)
	d := time.Since(t)

	return int(d.Seconds() / 60)
}

func (c *Context) tmplCurrentUserCreated() time.Time {
	t := bot.SnowflakeToTime(c.MS.ID)
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

func (c *Context) reFind(r string, s string) (string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return "", err
	}

	return compiled.FindString(s), nil
}

func (c *Context) reFindAll(r string, s string) ([]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	return compiled.FindAllString(s, 1000), nil
}

func (c *Context) reFindAllSubmatches(r string, s string) ([][]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	return compiled.FindAllStringSubmatch(s, 100), nil
}

func (c *Context) reReplace(r string, s string, repl string) (string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return "", err
	}

	return compiled.ReplaceAllString(s, repl), nil
}
