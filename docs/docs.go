package docs

//go:generate esc -o templates.go -pkg docs templates/

import (
	"bytes"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs/static"
	"github.com/shurcooL/github_flavored_markdown"
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
	common.RegisterPlugin(p)

	AddDefaultDoc()
}

type Page struct {
	Name   string
	Tmpl   *template.Template
	Static http.FileSystem
}

var (
	pages = make([]*Page, 0)
)

func FindPage(name string) *Page {
	for _, v := range pages {
		if strings.EqualFold(name, v.Name) {
			return v
		}
	}

	return nil
}

func AddPage(name string, content string, static http.FileSystem) {

	tmpl := AddTemplateFuncs(name, template.New("docs-"+name))
	tmpl = template.Must(tmpl.Parse(content))

	pages = append(pages, &Page{
		Name:   name,
		Tmpl:   tmpl,
		Static: static,
	})
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

		return strings.Replace(prefix+str, " ", "%20", -1)
	}
}

func AddDefaultDoc() {
	AddPage("Quickstart", FSMustString(false, "/templates/quickstart.md"), static.FS(false))
	AddPage("Helping Out", FSMustString(false, "/templates/helping-out.md"), static.FS(false))
	AddPage("Templates", FSMustString(false, "/templates/templates.md"), static.FS(false))
}

func (p *Page) Render() []byte {
	var buf bytes.Buffer
	p.Tmpl.Execute(&buf, nil)

	output := github_flavored_markdown.Markdown(buf.Bytes())
	return output
}
