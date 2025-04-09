package web

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
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

var permsString = map[string]int64{
	"ManageRoles":    discordgo.PermissionManageRoles,
	"ManageMessages": discordgo.PermissionManageMessages,
}

func hasPerm(botPerms int64, checkPerm string) (bool, error) {
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
func tmplRoleDropdown(roles []discordgo.Role, highestBotRole *discordgo.Role, args ...interface{}) template.HTML {
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
		output += `<optgroup label="────────────"></optgroup>`
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
			if common.IsRoleAbove(&role, highestBotRole) || role.ID == highestBotRole.ID {
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
func tmplRoleDropdownMulti(roles []discordgo.Role, highestBotRole *discordgo.Role, selections []int64) template.HTML {

	var builder strings.Builder

	// show deleted roles
OUTER:
	for _, sr := range selections {
		for _, gr := range roles {
			if sr == gr.ID {
				continue OUTER
			}
		}

		builder.WriteString(fmt.Sprintf(`<option value="%[1]d" selected>Deleted role: %[1]d</option>\n`, sr))
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
			if common.IsRoleAbove(&role, highestBotRole) || highestBotRole.ID == role.ID {
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

func tmplChannelOpts(channelTypes []discordgo.ChannelType) interface{} {
	optsBuilder := tmplChannelOptsMulti(channelTypes)
	return func(channels []dstate.ChannelState, selection interface{}, allowEmpty bool, emptyName string) template.HTML {

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

func tmplChannelOptsMulti(allowedChannelTypes []discordgo.ChannelType) func(channels []dstate.ChannelState, selections []int64) template.HTML {
	return func(channels []dstate.ChannelState, selections []int64) template.HTML {
		gen := &channelOptsHTMLGenState{allowedChannelTypes: allowedChannelTypes, channels: channels, selections: selections}
		return gen.HTML()
	}
}

type channelOptsHTMLGenState struct {
	allowedChannelTypes []discordgo.ChannelType
	channels            []dstate.ChannelState
	selections          []int64

	buf strings.Builder
}

func (g *channelOptsHTMLGenState) HTML() template.HTML {
	g.outputDeletedChannels()
	g.outputUncategorizedChannels()
	if len(g.allowedChannelTypes) > 1 || g.allowedChannelTypes[0] != discordgo.ChannelTypeGuildCategory {
		g.outputCategorizedChannels()
	}
	return template.HTML(g.buf.String())
}

func (g *channelOptsHTMLGenState) outputDeletedChannels() {
	exists := make(map[int64]bool)
	for _, c := range g.channels {
		exists[c.ID] = true
	}
	for _, sel := range g.selections {
		if !exists[sel] {
			g.output(fmt.Sprintf(`<option value="%[1]d" selected class="deleted-channel">Deleted channel: %[1]d</option>\n`, sel))
		}
	}
}

func (g *channelOptsHTMLGenState) outputUncategorizedChannels() {
	for _, c := range g.channels {
		if c.ParentID == 0 && g.include(c.Type) {
			g.outputChannel(c.ID, c.Name, c.Type)
		}
	}
}

func (g *channelOptsHTMLGenState) outputCategorizedChannels() {
	for _, cat := range g.channels {
		if cat.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}

		g.output(`<optgroup label="` + template.HTMLEscapeString(cat.Name) + `">`)
		for _, c := range g.channels {
			if c.ParentID == cat.ID && g.include(c.Type) {
				g.outputChannel(c.ID, c.Name, c.Type)
			}
		}
		g.output("</optgroup>")
	}
}

func (g *channelOptsHTMLGenState) include(channelType discordgo.ChannelType) bool {
	for _, t := range g.allowedChannelTypes {
		if t == channelType {
			return true
		}
	}
	return false
}

func (g *channelOptsHTMLGenState) outputChannel(id int64, name string, channelType discordgo.ChannelType) {
	g.output(`<option value="` + discordgo.StrID(id) + `"`)
	for _, selected := range g.selections {
		if selected == id {
			g.output(" selected")
		}
	}

	var prefix string
	switch channelType {
	case discordgo.ChannelTypeGuildText:
		prefix = "#"
	case discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildStageVoice:
		prefix = "🔊"
	case discordgo.ChannelTypeGuildForum:
		prefix = "📃"
	default:
		prefix = ""
	}
	g.output(">" + template.HTMLEscapeString(prefix+name) + "</option>")
}

func (g *channelOptsHTMLGenState) output(s string) {
	g.buf.WriteString(s)
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
	builder.WriteString(`<span class="tgl-desc ml-2 mb-2">` + description + `</span></div>`)
	// builder.WriteString(`</div>`)

	return template.HTML(builder.String())
}
