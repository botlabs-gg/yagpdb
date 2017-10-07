package templates

import (
	"fmt"
	"github.com/jonas747/discordgo"
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

		r := c.GS.Role(true, role)
		if r == nil {
			return "(role not found)"
		}

		if common.ContainsStringSlice(c.MentionRoles, role) {
			return "<@&" + role + ">"
		}

		c.MentionRoles = append(c.MentionRoles, role)
		return " <@&" + role + "> "
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
				if !common.ContainsStringSlice(c.MentionRoles, r.ID) {
					c.MentionRoles = append(c.MentionRoles, r.ID)
					found = r
				}
			}
		}
		c.GS.RUnlock()
		if found == nil {
			return "(role not found)"
		}

		return " <@&" + found.ID + "> "
	}
}

func tmplHasRoleID(c *Context) interface{} {
	numCalls := 0
	return func(roleID interface{}) bool {
		if numCalls >= 100 {
			return false
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
			return false
		}

		c.GS.RLock()
		contains := common.ContainsStringSlice(c.Member.Roles, role)
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
