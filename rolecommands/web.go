package rolecommands

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/src-d/go-kallax.v1"
	"html/template"
	"net/http"
)

type FormCommand struct {
	ID           int64
	Name         string `valid:",100"`
	Role         string `valid:"role,false`
	Group        int
	RequireRoles []string `valid:"role,true"`
	IgnoreRoles  []string `valid:"role,true"`
}

type FormGroup struct {
	ID           int64
	Name         string   `valid:",100"`
	RequireRoles []string `valid:"role,true"`
	IgnoreRoles  []string `valid:"role,true"`

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
	subMux.Handle(pat.Post("/remove_cmd"), web.ControllerPostHandler(HandleRemoveCommand, indexHandler, FormCommand{}, "Removed a role command"))

	subMux.Handle(pat.Post("/new_group"), web.ControllerPostHandler(HandleNewGroup, indexHandler, FormGroup{}, "Added a new role command group"))
	subMux.Handle(pat.Post("/update_group"), web.ControllerPostHandler(HandleUpdateGroup, indexHandler, FormGroup{}, "Updated a role command group"))
	subMux.Handle(pat.Post("/remove_group"), web.ControllerPostHandler(HandleRemoveGroup, indexHandler, FormGroup{}, "Removed a role command group"))
}

func HandleSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(r.Context())
	parsedGID := common.MustParseInt(g.ID)

	// groups, err := groupStore.FindAll(NewRoleGroupQuery().FindByGuildID(kallax.Eq, parsedGID))
	// if err != nil && err != kallax.ErrNotFound {
	// 	return tmpl, err
	// }

	commands, err := cmdStore.FindAll(NewRoleCommandQuery().FindByGuildID(kallax.Eq, parsedGID))
	if err != nil && err != kallax.ErrNotFound {
		return tmpl, err
	}

	// sortedCommands := make(map[RoleGroup][]*RoleCommand, len(groups))
	// for _, group := range groups {
	// 	sortedCommands[group] = make([]*RoleCommand, 0, 10)
	for _, cmd := range commands {
		logrus.Println(cmd.Group)
		// if cmd.Group {

		// }
	}
	// }

	return tmpl, nil
}

func HandleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
}

func HandleRemoveCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
}

func HandleNewGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
}

func HandleUpdateGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
}

func HandleRemoveGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return nil, nil
}
