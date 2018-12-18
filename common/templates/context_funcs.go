package templates

import (
	"errors"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var ErrTooManyCalls = errors.New("Too many calls to this function")

func (c *Context) tmplSendDM(s ...interface{}) string {
	if len(s) < 1 || c.IncreaseCheckCallCounter("send_dm", 1) {
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

func (c *Context) tmplSendMessage(filterSpecialMentions bool, returnID bool) func(channel interface{}, msg interface{}) interface{} {
	return func(channel interface{}, msg interface{}) interface{} {

		if c.IncreaseCheckCallCounter("send_message", 3) {
			return ""
		}

		var cid int64
		verifiedExistence := false

		c.GS.RLock()
		// Look for the channel
		if channel == nil && c.CS != nil {
			// No channel passed, assume current channel
			cid = c.CS.ID
		} else if channel != nil {
			switch t := channel.(type) {
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
		c.GS.RUnlock()

		if cid == 0 || !verifiedExistence {
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

func (c *Context) tmplMentionEveryone() string {
	c.MentionEveryone = true
	return " @everyone "
}

func (c *Context) tmplMentionHere() string {
	c.MentionHere = true
	return " @here "
}

func (c *Context) tmplMentionRoleID(roleID interface{}) string {
	if c.IncreaseCheckCallCounter("mention_role", 100) {
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

	r := c.GS.Role(true, role)
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
	if c.IncreaseCheckCallCounter("mention_role", 100) {
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
	if c.IncreaseCheckCallCounter("has_role", 200) {
		return false
	}

	role := ToInt64(roleID)
	if role == 0 {
		return false
	}

	c.GS.RLock()
	contains := common.ContainsInt64Slice(c.MS.Roles, role)
	c.GS.RUnlock()
	return contains
}

func (c *Context) tmplHasRoleName(name string) bool {
	if c.IncreaseCheckCallCounter("has_role", 200) {
		return false
	}

	c.GS.RLock()

	for _, r := range c.GS.Guild.Roles {
		if strings.EqualFold(r.Name, name) {
			if common.ContainsInt64Slice(c.MS.Roles, r.ID) {
				c.GS.RUnlock()
				return true
			}

			c.GS.RUnlock()
			return false
		}
	}

	// Role not found, default to false
	c.GS.RUnlock()
	return false
}

func targetUserID(input interface{}) int64 {
	switch t := input.(type) {
	case *discordgo.User:
		return t.ID
	default:
		return ToInt64(input)
	}
}

func (c *Context) tmplGiveRoleID(target interface{}, roleID interface{}) string {
	if c.IncreaseCheckCallCounter("add_role", 10) {
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
	if c.IncreaseCheckCallCounter("add_role", 10) {
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

func (c *Context) tmplTakeRoleID(target interface{}, roleID interface{}) string {
	if c.IncreaseCheckCallCounter("add_role", 10) {
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
	hasRole := true
	if ms != nil && ms.MemberSet {
		hasRole = common.ContainsInt64Slice(ms.Roles, role)
	}
	c.GS.RUnlock()

	if hasRole {
		common.BotSession.GuildMemberRoleRemove(c.GS.ID, targetID, role)
	}

	return ""
}

func (c *Context) tmplTakeRoleName(target interface{}, name string) string {
	if c.IncreaseCheckCallCounter("add_role", 10) {
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
			hasRole := true
			if ms != nil && ms.MemberSet {
				hasRole = common.ContainsInt64Slice(ms.Roles, role)
			}

			if !hasRole {
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

	common.BotSession.GuildMemberRoleRemove(c.GS.ID, targetID, role)

	return ""
}

func (c *Context) tmplAddRoleID(role interface{}) (string, error) {
	if c.IncreaseCheckCallCounter("add_role", 10) {
		return "", ErrTooManyCalls
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

func (c *Context) tmplRemoveRoleID(role interface{}) (string, error) {
	if c.IncreaseCheckCallCounter("remove_role", 10) {
		return "", ErrTooManyCalls
	}

	rid := ToInt64(role)
	if rid == 0 {
		return "", errors.New("No role id specified")
	}

	err := common.BotSession.GuildMemberRoleRemove(c.GS.ID, c.MS.ID, rid)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (c *Context) tmplDelResponse(args ...interface{}) string {
	dur := 10
	if len(args) > 0 {
		dur = int(ToInt64(args[0]))
	}
	if dur > 60 {
		dur = 60
	}

	c.DelResponseDelay = dur
	c.DelResponse = true
	return ""
}

func (c *Context) tmplDelTrigger(args ...interface{}) string {
	dur := 10
	if len(args) > 0 {
		dur = int(ToInt64(args[0]))
	}
	if dur > 60 {
		dur = 60
	}

	c.DelTriggerDelay = dur
	c.DelTrigger = true
	return ""
}

func (c *Context) tmplAddReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
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

	return callVariadic(f, values...)
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

	return callVariadic(f, values...)
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
