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

func tmplSendDM(c *Context) interface{} {
	return func(s ...interface{}) string {
		if c.SentDM {
			return ""
		}
		c.SentDM = true

		c.GS.RLock()
		gName := c.GS.Guild.Name
		memberID := c.Member.User.ID
		c.GS.RUnlock()

		msg := fmt.Sprint(s...)
		msg = fmt.Sprintf("Custom Command DM From the server **%s**:\n%s", gName, msg)
		bot.SendDM(memberID, msg)
		return ""
	}
}

func tmplMentionEveryone(c *Context) interface{} {
	return func() string {
		c.MentionEveryone = true
		return " @everyone "
	}
}

func tmplMentionHere(c *Context) interface{} {
	return func() string {
		c.MentionHere = true
		return " @here "
	}
}

func tmplMentionRoleID(c *Context) interface{} {
	numCalls := 0
	return func(roleID interface{}) string {
		if numCalls >= 50 {
			return ""
		}

		if len(c.MentionRoles) > 50 {
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
}

func tmplMentionRoleName(c *Context) interface{} {
	numCalls := 0
	return func(role string) string {
		if numCalls >= 50 {
			return ""
		}

		if len(c.MentionRoles) > 50 {
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
}

func tmplHasRoleID(c *Context) interface{} {
	numCalls := 0
	return func(roleID interface{}) bool {
		if numCalls >= 100 {
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
}

func tmplHasRoleName(c *Context) interface{} {
	numCalls := 0
	return func(name string) bool {
		if numCalls >= 100 {
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
}

func tmplAddRoleID(c *Context) interface{} {
	numCalls := 0
	return func(role interface{}) (string, error) {
		if numCalls >= 10 {
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
}

func tmplRemoveRoleID(c *Context) interface{} {
	numCalls := 0
	return func(role interface{}) (string, error) {
		if numCalls >= 10 {
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
}

func tmplDelResponse(c *Context) interface{} {
	return func() string {
		c.DelResponse = true
		return ""
	}
}

func tmplDelTrigger(c *Context) interface{} {
	return func() string {
		c.DelTrigger = true
		return ""
	}
}

func tmplAddReactions(c *Context) interface{} {
	return func(values ...reflect.Value) (reflect.Value, error) {
		f := func(args []reflect.Value) (reflect.Value, error) {
			for _, reaction := range args {
				if err := common.BotSession.MessageReactionAdd(c.Msg.ChannelID, c.Msg.ID, reaction.String()); err != nil {
					return reflect.Value{}, err
				}
			}
			return reflect.ValueOf(""), nil
		}
		return callVariadic(f, values...)
	}
}
