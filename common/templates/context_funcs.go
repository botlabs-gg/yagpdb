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
)

var ErrTooManyCalls = errors.New("Too many calls to this function")

func (c *Context) tmplSendDM(s ...interface{}) string {
	if c.IncreaseCheckCallCounter("send_dm", 1) {
		return ""
	}

	c.GS.RLock()
	gName := c.GS.Guild.Name
	memberID := c.Member.User.ID
	c.GS.RUnlock()

	msg := fmt.Sprint(s...)
	msg = fmt.Sprintf("Custom Command DM From the server **%s**:\n%s", gName, msg)
	bot.SendDM(memberID, msg)
	return ""
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

	var role int64
	switch r := roleID.(type) {
	case int64:
		role = r
	case int:
		role = int64(r)
	case string:
		role, _ = strconv.ParseInt(r, 10, 64)
	default:
		return false
	}

	c.GS.RLock()
	contains := common.ContainsInt64Slice(c.Member.Roles, role)
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
			if common.ContainsInt64Slice(c.Member.Roles, r.ID) {
				c.GS.RUnlock()
				return true
			}

			c.GS.RUnlock()
			return false
		}
	}

	c.GS.RUnlock()
	return true
}

func (c *Context) tmplAddRoleID(role interface{}) (string, error) {
	if c.IncreaseCheckCallCounter("add_role", 10) {
		return "", ErrTooManyCalls
	}

	rid := ToInt64(role)
	if rid == 0 {
		return "", errors.New("No role id specified")
	}

	err := common.BotSession.GuildMemberRoleAdd(c.GS.ID(), c.Member.User.ID, rid)
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

	err := common.BotSession.GuildMemberRoleRemove(c.GS.ID(), c.Member.User.ID, rid)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (c *Context) tmplDelResponse() string {
	c.DelResponse = true
	return ""
}

func (c *Context) tmplDelTrigger() string {
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
