package web

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/common/templates"
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
// roles: slice of roles to display options for
// highestBotRole: the bot's highest role, if not nil will disable roles above this one.
// args are optinal and in this order:
// 1. current selected roleid
// 2. default empty display name
// 3. default unknown display name
func tmplRoleDropdown(roles []*discordgo.Role, highestBotRole *discordgo.Role, args ...interface{}) template.HTML {
	hasCurrentSelected := len(args) > 0
	var currentSelected int64
	if hasCurrentSelected {
		currentSelected = templates.ToInt64(args[0])
	}

	hasEmptyName := len(args) > 1
	emptyName := ""
	if hasEmptyName {
		emptyName = templates.ToString(args[1])
	}

	hasUnknownName := len(args) > 2
	unknownName := "Unknown role (deleted most likely)"
	if hasUnknownName {
		emptyName = templates.ToString(args[2])
	}

	output := ""
	if hasEmptyName {
		output += `<option value=""`
		if currentSelected == 0 {
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
		if role.Managed && highestBotRole != nil {
			continue
		}

		output += `<option value="` + discordgo.StrID(role.ID) + `"`
		if role.ID == currentSelected {
			output += " selected"
			found = true
		}

		if role.Color != 0 {
			hexColor := fmt.Sprintf("%06x", int64(role.Color))
			output += " style=\"color: #" + hexColor + "\""
		}

		optName := template.HTMLEscapeString(role.Name)
		if highestBotRole != nil {
			if dutil.IsRoleAbove(role, highestBotRole) || role.ID == highestBotRole.ID {
				output += " disabled"
				optName += " (role is above bot)"
			}
		}
		output += ">" + optName + "</option>\n"
	}

	if !found && currentSelected != 0 {
		output += `<option value="` + discordgo.StrID(currentSelected) + `" selected>` + unknownName + "</option>\n"
	}

	return template.HTML(output)
}

// Same as tmplRoleDropdown but supports multiple selections
func tmplRoleDropdownMutli(roles []*discordgo.Role, highestBotRole *discordgo.Role, selections []int64) template.HTML {

	var builder strings.Builder

	// show deleted roles
OUTER:
	for _, sr := range selections {
		for _, gr := range roles {
			if sr == gr.ID {
				continue OUTER
			}
		}

		builder.WriteString(fmt.Sprintf(`<option value="0" selected>Deleted role: %d</option>\n`, sr))
	}

	for k, role := range roles {
		// Skip the everyone role
		if k == len(roles)-1 {
			break
		}

		// Allow the selection of managed roles in cases where we do not assign them (for filters and such for example)
		if role.Managed && highestBotRole != nil {
			continue
		}

		optIsSelected := false
		builder.WriteString(`<option value="` + discordgo.StrID(role.ID) + `"`)
		for _, selected := range selections {
			if selected == role.ID {
				builder.WriteString(" selected")
				optIsSelected = true
			}
		}

		if role.Color != 0 {
			hexColor := fmt.Sprintf("%06x", int64(role.Color))
			builder.WriteString(" data-color=\"#" + hexColor + "\"")
		}

		optName := template.HTMLEscapeString(role.Name)
		if highestBotRole != nil {
			if dutil.IsRoleAbove(role, highestBotRole) || highestBotRole.ID == role.ID {
				if !optIsSelected {
					builder.WriteString(" disabled")
				}

				optName += " (role is above bot)"
			}
		}
		builder.WriteString(">" + optName + "</option>\n")
	}

	return template.HTML(builder.String())
}

func tmplChannelOpts(channelTypes []discordgo.ChannelType, optionPrefix string) interface{} {
	optsBuilder := tmplChannelOptsMulti(channelTypes, optionPrefix)
	return func(channels []*discordgo.Channel, selection interface{}, allowEmpty bool, emptyName string) template.HTML {

		const unknownName = "Deleted channel"

		var builder strings.Builder

		if allowEmpty {
			if emptyName == "" {
				emptyName = "None"
			}

			builder.WriteString(`<option value=""`)
			if selection == 0 {
				builder.WriteString(" selected")
			}

			builder.WriteString(">" + template.HTMLEscapeString(emptyName) + "</option>")
		}

		var selections []int64
		intSel := templates.ToInt64(selection)
		if intSel != 0 {
			selections = []int64{intSel}
		}

		// Generate the rest of the options, which is the same as multi but with a single selections
		builder.WriteString(string(optsBuilder(channels, selections)))

		return template.HTML(builder.String())
	}
}

func tmplChannelOptsMulti(channelTypes []discordgo.ChannelType, optionPrefix string) func(channels []*discordgo.Channel, selections []int64) template.HTML {
	return func(channels []*discordgo.Channel, selections []int64) template.HTML {

		var builder strings.Builder

		channelOpt := func(id int64, name string) {
			builder.WriteString(`<option value="` + discordgo.StrID(id) + "\"")
			for _, selected := range selections {
				if selected == id {
					builder.WriteString(" selected")
				}
			}

			builder.WriteString(">" + template.HTMLEscapeString(name) + "</option>")
		}

		// Channels without a category
		for _, c := range channels {
			if c.ParentID != 0 || !containsChannelType(channelTypes, c.Type) {
				continue
			}

			channelOpt(c.ID, optionPrefix+c.Name)
		}

		// Group channels by category
		if len(channelTypes) > 1 || channelTypes[0] != discordgo.ChannelTypeGuildCategory {
			for _, cat := range channels {
				if cat.Type != discordgo.ChannelTypeGuildCategory {
					continue
				}

				builder.WriteString("<optgroup label=\"" + template.HTMLEscapeString(cat.Name) + "\">")
				for _, c := range channels {
					if !containsChannelType(channelTypes, c.Type) || c.ParentID != cat.ID {
						continue
					}

					channelOpt(c.ID, optionPrefix+c.Name)
				}
				builder.WriteString("</optgroup>")
			}
		}

		return template.HTML(builder.String())
	}
}

func containsChannelType(s []discordgo.ChannelType, t discordgo.ChannelType) bool {
	for _, v := range s {
		if v == t {
			return true
		}
	}

	return false
}

func tmplCheckbox(name, id, description string, checked bool, extraInputAttrs ...string) template.HTML {
	// 	const code = `<div class="checkbox-custom checkbox-primary">
	// 	<input type="checkbox" checked="" id="checkboxExample2">
	// 	<label for="checkboxExample2">Checkbox Primary</label>
	// </div>`

	// <input class="tgl tgl-flat" id="cb4" type="checkbox"/>
	// <label class="tgl-btn" for="cb4"></label>

	var builder strings.Builder
	builder.WriteString(`<div class="form-group d-flex align-items-center">`)
	builder.WriteString(`<input type="checkbox" class="tgl tgl-flat"`)
	builder.WriteString(` name="` + name + `" id="` + id + `"`)

	if checked {
		builder.WriteString(" checked")
	}
	if len(extraInputAttrs) > 0 {
		builder.WriteString(" " + strings.Join(extraInputAttrs, " "))
	}
	builder.WriteString(`><label for="` + id + `" class="tgl-btn mb-2"></label>`)
	// builder.WriteString(`><div class="switch"></div>`)
	builder.WriteString(`<span class="ml-2 mb-2">` + description + `</span></div>`)
	// builder.WriteString(`</div>`)

	return template.HTML(builder.String())
}
