package web

import (
	"bytes"
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/common/templates"
	"html/template"
	"time"
)

func prettyTime(t time.Time) string {
	return t.Format(time.RFC822)
}

// mTemplate combines "template" with dictionary. so you can specify multiple variables
// and call templates almost as if they were functions with arguments
// makes certain templates a lot simpler
func mTemplate(name string, values ...interface{}) (template.HTML, error) {

	data, err := templates.Dictionary(values...)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = Templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", err
	}

	return template.HTML(buf.String()), nil
}

var permsString = map[string]int{
	"ManageRoles":    discordgo.PermissionManageRoles,
	"ManageMessages": discordgo.PermissionManageMessages,
}

func hasPerm(botPerms int, checkPerm string) (bool, error) {
	p, ok := permsString[checkPerm]
	if !ok {
		return false, errors.New("Unknown permission: " + checkPerm)
	}

	return botPerms&p != 0, nil
}

// tmplRoleDropdown is a template function for generating role dropdown options
// args are optinal and in this order:
// current selected roleid
// default empty display name
// default unknown display name
func tmplRoleDropdown(roles []*discordgo.Role, highestBotRole *discordgo.Role, args ...string) template.HTML {
	hasCurrentSelected := len(args) > 0
	currentSelected := ""
	if hasCurrentSelected {
		currentSelected = args[0]
	}

	hasEmptyName := len(args) > 1
	emptyName := ""
	if hasEmptyName {
		emptyName = args[1]
	}

	hasUnknownName := len(args) > 2
	unknownName := "Unknown role (deleted most likely)"
	if hasUnknownName {
		emptyName = args[2]
	}

	output := ""
	if hasEmptyName {
		output += `<option value=""`
		if currentSelected == "" {
			output += `selected`
		}
		output += ">" + template.HTMLEscapeString(emptyName) + "</option>\n"
	}

	found := false
	for k, role := range roles {
		// Skip the everyone role
		if k == len(roles)-1 {
			break
		}
		if role.Managed {
			continue
		}

		output += `<option value="` + role.ID + `"`
		if role.ID == currentSelected {
			output += " selected"
			found = true
		}

		optName := template.HTMLEscapeString(role.Name)
		if highestBotRole != nil {
			if dutil.IsRoleAbove(role, highestBotRole) {
				output += " disabled"
				optName += " (role is above bot)"
			}
		}
		output += ">" + optName + "</option>\n"
	}

	if !found && currentSelected != "" {
		output += `<option value="` + currentSelected + `" selected>` + unknownName + "</option>\n"
	}

	return template.HTML(output)
}
