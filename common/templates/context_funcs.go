package templates

import (
	"fmt"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"strings"
)

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
		return ""
	}
}

func tmplMentionHere(c *Context) interface{} {
	return func() string {
		c.MentionHere = true
		return ""
	}
}

func tmplMentionRoleID(c *Context) interface{} {
	return func(roleID interface{}) string {
		if len(c.MentionRoles) > 50 {
			return ""
		}

		role := ""
		switch r := roleID.(type) {
		case int64:
			role = strconv.FormatInt(r, 10)
		case int32:
			role = strconv.FormatInt(int64(r), 10)
		case int:
			role = strconv.FormatInt(int64(r), 10)
		case string:
			role = r
		default:
			return ""
		}

		c.MentionRoles = append(c.MentionRoles, role)
		return ""
	}
}

func tmplMentionRoleName(c *Context) interface{} {
	return func(role string) string {
		if len(c.MentionRoleNames) > 50 {
			return ""
		}

		c.MentionRoleNames = append(c.MentionRoleNames, role)
		return ""
	}
}

func tmplHasRoleID(c *Context) interface{} {
	return func(roleID interface{}) bool {
		role := ""
		switch r := roleID.(type) {
		case int64:
			role = strconv.FormatInt(r, 10)
		case int32:
			role = strconv.FormatInt(int64(r), 10)
		case int:
			role = strconv.FormatInt(int64(r), 10)
		case string:
			role = r
		default:
			return false
		}

		c.GS.RLock()
		contains := common.ContainsStringSlice(c.Member.Roles, role)
		c.GS.RUnlock()
		return contains
	}
}

func tmplHasRoleName(c *Context) interface{} {
	return func(name string) bool {
		c.GS.RLock()

		for _, r := range c.GS.Guild.Roles {
			if strings.EqualFold(r.Name, name) {
				if common.ContainsStringSlice(c.Member.Roles, r.ID) {
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
