package rolecommands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/src-d/go-kallax.v1"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type FormCommand struct {
	ID           int64
	Name         string `valid:"1,100,trimspace"`
	Role         int64  `valid:"role,false`
	Group        int64
	RequireRoles []int64 `valid:"role,true"`
	IgnoreRoles  []int64 `valid:"role,true"`
}

type FormGroup struct {
	ID           int64
	Name         string  `valid:"1,100,trimspace"`
	RequireRoles []int64 `valid:"role,true"`
	IgnoreRoles  []int64 `valid:"role,true"`

	Mode int

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
	}

	tmpl["SortedCommands"] = sortedCommands

	loneCommands := make([]*RoleCommand, 0, 10)
	for _, cmd := range commands {
		if cmd.Group == nil {
			loneCommands = append(loneCommands, cmd)
		}
	}
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

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
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
	}
	err := groupStore.Insert(model)
	if err != nil {
		return tmpl, err
	}

	return tmpl, nil
}

func HandleUpdateGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
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
