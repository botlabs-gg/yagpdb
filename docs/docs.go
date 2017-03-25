package docs

//go:generate esc -o templates.go -pkg docs templates/

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs/static"
	"github.com/jonas747/yagpdb/web"
	"net/http"
	"strings"
	"text/template"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Docs"
}

func RegisterPlugin() {
	p := &Plugin{}
	web.RegisterPlugin(p)

	AddDefaultDoc()
}

type Page struct {
	Name   string
	Tmpl   *template.Template
	Static http.FileSystem
}

var (
	pages = make(map[string]*Page)
)

func AddPage(name string, content string, static http.FileSystem) {

	tmpl := AddTemplateFuncs(name, template.New("docs-"+name))
	tmpl = template.Must(tmpl.Parse(content))

	pages[name] = &Page{
		Name:   name,
		Tmpl:   tmpl,
		Static: static,
	}
}

func AddTemplateFuncs(page string, tmpl *template.Template) *template.Template {
	return tmpl.Funcs(template.FuncMap{
		"static": staticContent(page),
	})
}

func staticContent(page string) func(string) string {
	prefix := "https://" + common.Conf.Host + "/staticdocs/" + page
	return func(str string) string {
		if !strings.HasPrefix(str, "/") {
			str = "/" + str
		}

		return prefix + str
	}
}

func AddDefaultDoc() {
	AddPage("Quickstart", FSMustString(false, "/templates/quickstart.md"), static.FS(false))
	AddPage("Helping Out", FSMustString(false, "/templates/helping-out.md"), static.FS(false))
}
