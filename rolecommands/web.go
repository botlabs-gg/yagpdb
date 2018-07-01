package rolecommands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/rolecommands/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/volatiletech/null.v6"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type FormCommand struct {
	ID           int64
	Name         string `valid:",1,100,trimspace"`
	Role         int64  `valid:"role,false"`
	Group        int64
	RequireRoles []int64 `valid:"role,true"`
	IgnoreRoles  []int64 `valid:"role,true"`
}

type FormGroup struct {
	ID           int64
	Name         string  `valid:",1,100,trimspace"`
	RequireRoles []int64 `valid:"role,true"`
	IgnoreRoles  []int64 `valid:"role,true"`

	Mode int

	MultipleMax int
	MultipleMin int

	SingleAutoToggleOff bool
	SingleRequireOne    bool
}

func (p *Plugin) InitWeb() {
	tmplPathSettings := "templates/plugins/rolecommands.html"
	if common.Testing {
		tmplPathSettings = "../../rolecommands/assets/rolecommands.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings))

	// Setup SubMuxer
	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/rolecommands/*"), subMux)
	web.CPMux.Handle(pat.New("/rolecommands"), subMux)

	subMux.Use(web.RequireFullGuildMW)
	subMux.Use(web.RequireBotMemberMW)
	subMux.Use(web.RequirePermMW(discordgo.PermissionManageRoles))

	// Setup routes
	getIndexHandler := web.ControllerHandler(HandleGetIndex, "cp_rolecommands")
	getGroupHandler := web.ControllerHandler(func(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
		groupIDRaw := pat.Param(r, "groupID")
		parsed, _ := strconv.ParseInt(groupIDRaw, 10, 64)
		return HandleGetGroup(parsed, w, r)
	}, "cp_rolecommands")

	subMux.Handle(pat.Get("/"), getIndexHandler)
	subMux.Handle(pat.Get("/group/:groupID"), getGroupHandler)

	// either serve the group page or the index page, kinda convoluted but eh
	getIndexpPostHandler := web.ControllerHandler(func(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
		if r.FormValue("GroupID") != "" {
			parsed, _ := strconv.ParseInt(r.FormValue("GroupID"), 10, 64)
			return HandleGetGroup(parsed, w, r)
		}

		if r.FormValue("Group") != "" {
			parsed, _ := strconv.ParseInt(r.FormValue("Group"), 10, 64)
			return HandleGetGroup(parsed, w, r)
		}

		_, _, tmpl = web.GetBaseCPContextData(r.Context())
		if idInterface, ok := tmpl["GroupID"]; ok {
			return HandleGetGroup(idInterface.(int64), w, r)
		}

		return HandleGetIndex(w, r)
	}, "cp_rolecommands")

	subMux.Handle(pat.Post("/new_cmd"), web.ControllerPostHandler(HandleNewCommand, getIndexpPostHandler, FormCommand{}, "Added a new role command"))
	subMux.Handle(pat.Post("/update_cmd"), web.ControllerPostHandler(HandleUpdateCommand, getIndexpPostHandler, FormCommand{}, "Updated a role command"))
	subMux.Handle(pat.Post("/remove_cmd"), web.ControllerPostHandler(HandleRemoveCommand, getIndexpPostHandler, nil, "Removed a role command"))
	subMux.Handle(pat.Post("/move_cmd"), web.ControllerPostHandler(HandleMoveCommand, getIndexpPostHandler, nil, "Moved a role command"))

	subMux.Handle(pat.Post("/new_group"), web.ControllerPostHandler(HandleNewGroup, getIndexpPostHandler, FormGroup{}, "Added a new role command group"))
	subMux.Handle(pat.Post("/update_group"), web.ControllerPostHandler(HandleUpdateGroup, getIndexpPostHandler, FormGroup{}, "Updated a role command group"))
	subMux.Handle(pat.Post("/remove_group"), web.ControllerPostHandler(HandleRemoveGroup, getIndexpPostHandler, nil, "Removed a role command group"))
}

func HandleGetIndex(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	ungroupedCommands, err := models.RoleCommandsG(qm.Where("guild_id = ?", g.ID), qm.Where("role_group_id is null")).All()
	if err != nil {
		return tmpl, err
	}
	sort.Slice(ungroupedCommands, RoleCommandsLessFunc(ungroupedCommands))

	tmpl["LoneCommands"] = ungroupedCommands

	groups, err := models.RoleGroupsG(qm.Where(models.RoleGroupColumns.GuildID+" = ?", g.ID), qm.OrderBy("id asc")).All()
	if err != nil {
		return tmpl, err
	}

	tmpl["Groups"] = groups

	return tmpl, nil
}

func HandleGetGroup(groupID int64, w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	groups, err := models.RoleGroupsG(qm.Where(models.RoleGroupColumns.GuildID+" = ?", g.ID), qm.OrderBy("id asc")).All()
	if err != nil {
		return tmpl, err
	}

	tmpl["Groups"] = groups

	var currentGroup *models.RoleGroup
	for _, v := range groups {
		if v.ID == groupID {
			tmpl["CurrentGroup"] = v
			currentGroup = v
			break
		}
	}

	if currentGroup != nil {
		commands, err := currentGroup.RoleCommandsG().All()
		if err != nil {
			return tmpl, err
		}
		sort.Slice(commands, RoleCommandsLessFunc(commands))

		tmpl["Commands"] = commands
	} else {
		// Fallback
		return HandleGetIndex(w, r)
	}

	return tmpl, nil
}

func HandleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	form := r.Context().Value(common.ContextKeyParsedForm).(*FormCommand)
	form.Name = strings.TrimSpace(form.Name)

	if c, _ := models.RoleCommandsG(qm.Where(models.RoleCommandColumns.GuildID+"=?", g.ID)).Count(); c >= 1000 {
		tmpl.AddAlerts(web.ErrorAlert("Max 1000 role commands allowed"))
		return tmpl, nil
	}

	if c, _ := models.RoleCommandsG(qm.Where(models.RoleCommandColumns.GuildID+"=?", g.ID), qm.Where(models.RoleCommandColumns.Name+" ILIKE ?", form.Name)).Count(); c > 0 {
		tmpl.AddAlerts(web.ErrorAlert("Already a role command with that name"))
		return tmpl, nil
	}

	model := &models.RoleCommand{
		Name:    form.Name,
		GuildID: g.ID,

		Role:         form.Role,
		RequireRoles: form.RequireRoles,
		IgnoreRoles:  form.IgnoreRoles,
	}

	if form.Group != -1 {
		group, err := models.RoleGroupsG(qm.Where(models.RoleGroupColumns.GuildID+"=?", g.ID), qm.Where(models.RoleGroupColumns.ID+"=?", form.Group)).One()
		if err != nil {
			return tmpl, err
		}

		model.RoleGroupID = null.Int64From(group.ID)
	}

	err := model.InsertG()

	return tmpl, err
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	formCmd := r.Context().Value(common.ContextKeyParsedForm).(*FormCommand)

	cmd, err := models.FindRoleCommandG(formCmd.ID)
	if err != nil {
		return
	}

	if cmd.GuildID != g.ID {
		return tmpl.AddAlerts(web.ErrorAlert("That's not your command")), nil
	}

	cmd.Name = formCmd.Name
	cmd.Role = formCmd.Role
	cmd.IgnoreRoles = formCmd.IgnoreRoles
	cmd.RequireRoles = formCmd.RequireRoles

	if formCmd.Group != -1 {
		group, err := models.FindRoleGroupG(formCmd.Group)
		if err != nil {
			return tmpl, err
		}
		if group.GuildID != g.ID {
			return tmpl.AddAlerts(web.ErrorAlert("That's not your group")), nil
		}
		err = cmd.SetRoleGroupG(false, group)
		if err != nil {
			return tmpl, err
		}
	} else {
		cmd.RoleGroupID.Valid = false
		if err = cmd.UpdateG(models.RoleCommandColumns.RoleGroupID); err != nil {
			cmd.RoleGroupID.Valid = true
			return tmpl, err
		}
	}

	err = cmd.UpdateG(models.RoleCommandColumns.Name, models.RoleCommandColumns.Role, models.RoleCommandColumns.IgnoreRoles, models.RoleCommandColumns.RequireRoles)
	return
}

func HandleMoveCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	commands, err := models.RoleCommandsG(qm.Where("guild_id=?", g.ID)).All()
	if err != nil {
		return tmpl, err
	}

	tID, err := strconv.ParseInt(r.FormValue("ID"), 10, 32)
	if err != nil {
		return tmpl, err
	}

	var targetCmd *models.RoleCommand
	for _, v := range commands {
		if v.ID == tID {
			targetCmd = v
			break
		}
	}

	if targetCmd == nil {
		return tmpl, errors.New("RoleCommand not found")
	}

	commandsInGroup := make([]*models.RoleCommand, 0, len(commands))

	// Sort all relevant commands
	for _, v := range commands {
		if (!targetCmd.RoleGroupID.Valid && !v.RoleGroupID.Valid) || (targetCmd.RoleGroupID.Valid && v.RoleGroupID.Valid && targetCmd.RoleGroupID.Int64 == v.RoleGroupID.Int64) {
			commandsInGroup = append(commandsInGroup, v)
		}
	}

	sort.Slice(commandsInGroup, RoleCommandsLessFunc(commandsInGroup))

	isUp := r.FormValue("dir") == "1"

	// Move the position
	for i := 0; i < len(commandsInGroup); i++ {
		v := commandsInGroup[i]

		v.Position = int64(i)
		if v.ID == tID {
			if isUp {
				if i == 0 {
					// Can't move further up
					continue
				}

				v.Position--
				commandsInGroup[i-1].Position = int64(i)
			} else {
				if i == len(commandsInGroup)-1 {
					// Can't move further down
					continue
				}
				v.Position++
				commandsInGroup[i+1].Position = int64(i)
				i++
			}
		}
	}

	for _, v := range commandsInGroup {
		lErr := v.UpdateG(models.RoleCommandColumns.Position)
		if lErr != nil {
			err = lErr
		}
	}

	return tmpl, err
}

func HandleRemoveCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	idParsed, _ := strconv.ParseInt(r.FormValue("ID"), 10, 64)
	err := models.RoleCommandsG(qm.Where("guild_id=?", g.ID), qm.Where("id=?", idParsed)).DeleteAll()
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func HandleNewGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	form := r.Context().Value(common.ContextKeyParsedForm).(*FormGroup)
	form.Name = strings.TrimSpace(form.Name)

	if c, _ := models.RoleGroupsG(qm.Where("guild_id=?", g.ID)).Count(); c >= 1000 {
		tmpl.AddAlerts(web.ErrorAlert("Max 1000 role groups allowed"))
		return tmpl, nil
	}

	if c, _ := models.RoleGroupsG(qm.Where("guild_id=?", g.ID), qm.Where("name ILIKE ?", form.Name)).Count(); c > 0 {
		tmpl.AddAlerts(web.ErrorAlert("Already a role group with that name"))
		return tmpl, nil
	}

	model := &models.RoleGroup{
		Name:    form.Name,
		GuildID: g.ID,

		RequireRoles: form.RequireRoles,
		IgnoreRoles:  form.IgnoreRoles,
		Mode:         int64(form.Mode),

		MultipleMax:         int64(form.MultipleMax),
		MultipleMin:         int64(form.MultipleMin),
		SingleRequireOne:    form.SingleRequireOne,
		SingleAutoToggleOff: form.SingleAutoToggleOff,
	}

	err := model.InsertG()
	if err != nil {
		return tmpl, err
	}

	tmpl["GroupID"] = model.ID

	return tmpl, nil
}

func HandleUpdateGroup(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())

	formGroup := r.Context().Value(common.ContextKeyParsedForm).(*FormGroup)

	group, err := models.RoleGroupsG(qm.Where("guild_id=?", g.ID), qm.Where("id=?", formGroup.ID)).One()
	if err != nil {
		return
	}

	group.Name = formGroup.Name
	group.IgnoreRoles = formGroup.IgnoreRoles
	group.RequireRoles = formGroup.RequireRoles
	group.SingleRequireOne = formGroup.SingleRequireOne
	group.SingleAutoToggleOff = formGroup.SingleAutoToggleOff
	group.MultipleMax = int64(formGroup.MultipleMax)
	group.MultipleMin = int64(formGroup.MultipleMin)
	group.Mode = int64(formGroup.Mode)

	tmpl["GroupID"] = group.ID

	err = group.UpdateG()
	return
}

func HandleRemoveGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, _ := web.GetBaseCPContextData(r.Context())

	idParsed, _ := strconv.ParseInt(r.FormValue("ID"), 10, 64)
	err := models.RoleGroupsG(qm.Where("guild_id=?", g.ID), qm.Where("id=?", idParsed)).DeleteAll()
	return nil, err
}
