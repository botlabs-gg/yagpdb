package docs

import (
	"bytes"
	"github.com/jonas747/yagpdb/web"
	"github.com/russross/blackfriday"
	"goji.io/pat"
	"html/template"
	"net/http"
	"path"
	"strings"
)

func (d *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/templates/docs.ghtml")))

	web.AddGlobalTemplateData("DocPages", pages)

	web.RootMux.Handle(pat.Get("/docs/:page"), web.ControllerHandler(PageHandler, "docs-page"))
	web.RootMux.HandleFunc(pat.Get("/staticdocs/:page/*"), StaticHandler)
}

var (
	renderer = blackfriday.HtmlRenderer(0, "Test", "")
)

func PageHandler(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	page, ok := pages[pat.Param(r, "page")]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Page not found"))
		return
	}

	// first render the template
	var buf bytes.Buffer
	page.Tmpl.Execute(&buf, nil)

	output := blackfriday.Markdown(buf.Bytes(), renderer, 0)
	_, tmpl = web.GetCreateTemplateData(r.Context())
	tmpl["CurrentDocPage"] = page
	tmpl["DocContent"] = template.HTML(output)

	return nil, nil
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	pageName := pat.Param(r, "page")

	page, ok := pages[pageName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Page not found"))
		return
	}

	if page.Static == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Page has no static files"))
		return
	}

	upath := r.URL.Path

	toStrip := len("/staticdocs/Quickstart")
	if !strings.HasPrefix(upath, "/") {
		toStrip--
	}
	upath = upath[toStrip:]

	f, err := page.Static.Open(path.Clean(upath))
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).Error("Failed serving file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stat, _ := f.Stat()

	http.ServeContent(w, r, page.Name, stat.ModTime(), f)
}
