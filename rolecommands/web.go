package rolecommands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"github.com/pkg/errors"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/src-d/go-kallax.v1"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type FormCommand struct {
	ID           int64
	Name         string `valid:",1,100,trimspace"`
	Role         int64  `valid:"role,false`
	Group        int64
	RequireRoles []int64 `valid:"role,true"`
	IgnoreRoles  []int64 `valid:"role,true"`
}

type FormGroup struct {
	ID           int64
	Name         string  `valid:",1,100,trimspace"`
	RequireRoles []int64 `valid:"role,true"`
	IgnoreRoles  []int64 `valid:"role,true"`

	Mode GroupMode

	MultipleMax int
	MultipleMin int

	SingleAutoToggleOff bool
	SingleRequireOne    bool
}

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/assets/settings.html")))

	// Setup SubMuxer
	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/rolecommands/*"), subMux)
	web.CPMux.Handle(pat.New("/rolecommands"), subMux)

	subMux.Use(web.RequireFullGuildMW)
	subMux.Use(web.RequireBotMemberMW)
	subMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles))

	// Setup routes
	indexHandler := web.ControllerHandler(HandleSettings, "cp_rolecommands")

	subMux.Handle(pat.Get("/"), indexHandler)

	subMux.Handle(pat.Post("/new_cmd"), web.ControllerPostHandler(HandleNewCommand, indexHandler, FormCommand{}, "Added a new role command"))
	subMux.Handle(pat.Post("/update_cmd"), web.ControllerPostHandler(HandleUpdateCommand, indexHandler, FormCommand{}, "Updated a role command"))
	subMux.Handle(pat.Post("/remove_cmd"), web.ControllerPostHandler(HandleRemoveCommand, indexHandler, nil, "Removed a role command"))
	subMux.Handle(pat.Post("/move_cmd"), web.ControllerPostHandler(HandleMoveCommand, indexHandler, nil, "Moved a role command"))

	subMux.Handle(pat.Post("/new_group"), web.ControllerPostHandler(HandleNewGroup, indexHandler, FormGroup{}, "Added a new role command group"))
	subMux.Handle(pat.Post("/update_group"), web.ControllerPostHandler(HandleUpdateGroup, indexHandler, FormGroup{}, "Updated a role command group"))
	subMux.Handle(pat.Post("/remove_group"), web.ControllerPostHandler(HandleRemoveGroup, indexHandler, nil, "Removed a role command group"))
}

func HandleSettings(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	parsedGID := common.MustParseInt(g.ID)

	groups, err := groupStore.FindAll(NewRoleGroupQuery().FindByGuildID(kallax.Eq, parsedGID))
	if err != nil && err != kallax.ErrNotFound {
		return tmpl, err
	}

	tmpl["Groups"] = groups

	commands, err := cmdStore.FindAll(NewRoleCommandQuery().WithGroup())
	if err != nil && err != kallax.ErrNotFound {
		return tmpl, err
	}

	sortedCommands := make(map[*RoleGroup][]*RoleCommand, len(groups))
	for _, group := range groups {
		sortedCommands[group] = make([]*RoleCommand, 0, 10)
		for _, cmd := range commands {
			if cmd.Group != nil && cmd.Group.ID == group.ID {
				sortedCommands[group] = append(sortedCommands[group], cmd)
			}
		}

		sort.Slice(sortedCommands[group], RoleCommandsLessFunc(sortedCommands[group]))
	}

	tmpl["SortedCommands"] = sortedCommands

	loneCommands := make([]*RoleCommand, 0, 10)
	for _, cmd := range commands {
		if cmd.Group == nil {
			loneCommands = append(loneCommands, cmd)
		}
	}
	sort.Slice(loneCommands, RoleCommandsLessFunc(loneCommands))
	tmpl["LoneCommands"] = loneCommands

	return tmpl, nil
}

func HandleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	parsedGID := common.MustParseInt(g.ID)

	form := r.Context().Value(common.ContextKeyParsedForm).(*FormCommand)
	form.Name = strings.TrimSpace(form.Name)
	if c, _ := cmdStore.Count(NewRoleCommandQuery().FindByGuildID(kallax.Eq, parsedGID)); c >= 1000 {
		tmpl.AddAlerts(web.ErrorAlert("Max 1000 role commands allowed"))
		return tmpl, nil
	}

	if c, _ := cmdStore.Count(NewRoleCommandQuery().FindByGuildID(kallax.Eq, parsedGID).Where(kallax.Ilike(Schema.RoleCommand.Name, form.Name))); c > 0 {
		tmpl.AddAlerts(web.ErrorAlert("Already a role command with that name"))
		return tmpl, nil
	}

	model := &RoleCommand{
		Name:    form.Name,
		GuildID: parsedGID,

		Role:         form.Role,
		RequireRoles: form.RequireRoles,
		IgnoreRoles:  form.IgnoreRoles,
	}

	if form.Group != -1 {
		group, err := groupStore.FindOne(NewRoleGroupQuery().FindByGuildID(kallax.Eq, common.MustParseInt(g.ID)).FindByID(form.Group))
		if err != nil {
			return tmpl, err
		}

		model.Group = group
	}

	err := cmdStore.Insert(model)
	if err != nil {
		return tmpl, err
	}

	return tmpl, nil
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	formCmd := r.Context().Value(common.ContextKeyParsedForm).(*FormCommand)

	cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByGuildID(kallax.Eq, common.MustParseInt(g.ID)).FindByID(formCmd.ID))
	if err != nil {
		return
	}

	cmd.Name = formCmd.Name
	cmd.Role = formCmd.Role
	cmd.IgnoreRoles = formCmd.IgnoreRoles
	cmd.RequireRoles = formCmd.RequireRoles

	if formCmd.Group != 0 {
		group, err := groupStore.FindOne(NewRoleGroupQuery().FindByGuildID(kallax.Eq, cmd.GuildID).FindByID(formCmd.Group))
		if err != nil && err != kallax.ErrNotFound {
			return tmpl, err
		}
		cmd.Group = group
	} else {
		cmd.Group = nil
	}

	_, err = cmdStore.Update(cmd)
	return
}

func HandleMoveCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	commands, err := cmdStore.FindAll(NewRoleCommandQuery().FindByGuildID(kallax.Eq, common.MustParseInt(g.ID)).WithGroup())
	if err != nil {
		return tmpl, err
	}

	tID, err := strconv.ParseInt(r.FormValue("ID"), 10, 32)
	if err != nil {
		return tmpl, err
	}

	var targetCmd *RoleCommand
	for _, v := range commands {
		if v.ID == tID {
			targetCmd = v
			break
		}
	}

	if targetCmd == nil {
		return tmpl, errors.New("RoleCommand not found")
	}

	commandsInGroup := make([]*RoleCommand, 0, len(commands))

	// Sort all relevant commands
	for _, v := range commands {
		if (targetCmd.Group == nil && v.Group == nil) || (targetCmd.Group != nil && v.Group != nil && targetCmd.Group.ID == v.Group.ID) {
			commandsInGroup = append(commandsInGroup, v)
		}
	}

	sort.Slice(commandsInGroup, RoleCommandsLessFunc(commandsInGroup))

	isUp := r.FormValue("dir") == "1"

	// Move the position
	for i := 0; i < len(commandsInGroup); i++ {
		v := commandsInGroup[i]

		v.Position = i
		if v.ID == tID {
			if isUp {
				if i == 0 {
					// Can't move further up
					continue
				}

				v.Position--
				commandsInGroup[i-1].Position = i
			} else {
				if i == len(commandsInGroup)-1 {
					// Can't move further down
					continue
				}
				v.Position++
				commandsInGroup[i+1].Position = i
				i++
			}
		}
	}

	for _, v := range commandsInGroup {
		_, lErr := cmdStore.Update(v, Schema.RoleCommand.Position)
		if lErr != nil {
			err = lErr
		}
	}

	return tmpl, err
}

func HandleRemoveCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, _ := web.GetBaseCPContextData(r.Context())

	idParsed, _ := strconv.ParseInt(r.FormValue("ID"), 10, 64)
	cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByGuildID(kallax.Eq, common.MustParseInt(g.ID)).FindByID(idParsed))
	if err != nil {
		return nil, err
	}

	err = cmdStore.Delete(cmd)
	return nil, err
}

func HandleNewGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	parsedGID := common.MustParseInt(g.ID)

	form := r.Context().Value(common.ContextKeyParsedForm).(*FormGroup)
	form.Name = strings.TrimSpace(form.Name)
	if c, _ := groupStore.Count(NewRoleGroupQuery().FindByGuildID(kallax.Eq, parsedGID)); c >= 1000 {
		tmpl.AddAlerts(web.ErrorAlert("Max 1000 role groups allowed"))
		return tmpl, nil
	}

	if c, _ := groupStore.Count(NewRoleGroupQuery().FindByGuildID(kallax.Eq, parsedGID).Where(kallax.Ilike(Schema.RoleGroup.Name, form.Name))); c > 0 {
		tmpl.AddAlerts(web.ErrorAlert("Already a role group with that name"))
		return tmpl, nil
	}

	model := &RoleGroup{
		Name:    form.Name,
		GuildID: parsedGID,

		RequireRoles: form.RequireRoles,
		IgnoreRoles:  form.IgnoreRoles,
		Mode:         form.Mode,

		MultipleMax:         form.MultipleMax,
		MultipleMin:         form.MultipleMin,
		SingleRequireOne:    form.SingleRequireOne,
		SingleAutoToggleOff: form.SingleAutoToggleOff,
	}
	err := groupStore.Insert(model)
	if err != nil {
		return tmpl, err
	}

	return tmpl, nil
}

func HandleUpdateGroup(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	formGroup := r.Context().Value(common.ContextKeyParsedForm).(*FormGroup)

	group, err := groupStore.FindOne(NewRoleGroupQuery().FindByGuildID(kallax.Eq, common.MustParseInt(g.ID)).FindByID(formGroup.ID))
	if err != nil {
		return
	}

	group.Name = formGroup.Name
	group.IgnoreRoles = formGroup.IgnoreRoles
	group.RequireRoles = formGroup.RequireRoles
	group.SingleRequireOne = formGroup.SingleRequireOne
	group.SingleAutoToggleOff = formGroup.SingleAutoToggleOff
	group.MultipleMax = formGroup.MultipleMax
	group.MultipleMin = formGroup.MultipleMin
	group.Mode = formGroup.Mode

	_, err = groupStore.Update(group)
	return
}

func HandleRemoveGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, _ := web.GetBaseCPContextData(r.Context())

	idParsed, _ := strconv.ParseInt(r.FormValue("ID"), 10, 64)
	group, err := groupStore.FindOne(NewRoleGroupQuery().FindByGuildID(kallax.Eq, common.MustParseInt(g.ID)).FindByID(idParsed))
	if err != nil {
		return nil, err
	}

	err = groupStore.Delete(group)
	return nil, err
}
