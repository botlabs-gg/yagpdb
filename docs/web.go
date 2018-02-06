package docs

import (
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func (d *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/templates/docs.ghtml")))

	web.AddGlobalTemplateData("DocPages", pages)

	web.RootMux.Handle(pat.Get("/docs/:page"), web.ControllerHandler(PageHandler, "docs-page"))
	web.RootMux.HandleFunc(pat.Get("/staticdocs/:page/*"), StaticHandler)
}

// var (
// 	renderer = blackfriday.HtmlRenderer(0, "Test", "")
// )

func PageHandler(w http.ResponseWriter, r *http.Request) (tmpl web.TemplateData, err error) {
	name, err := url.QueryUnescape(pat.Param(r, "page"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Page not found"))
		return
	}

	page := FindPage(name)
	if page == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Page not found"))
		return
	}

	_, tmpl = web.GetCreateTemplateData(r.Context())

	// first render the template
	tmpl["CurrentDocPage"] = page

	renderd := page.Render()
	tmpl["DocContent"] = template.HTML(renderd)

	return nil, nil
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	pageName, err := url.QueryUnescape(pat.Param(r, "page"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Page not found"))
		return
	}

	page := FindPage(pageName)
	if page == nil {
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

	toStrip := len("/staticdocs/") + len(pageName) + 1
	if !strings.HasPrefix(upath, "/") {
		toStrip--
	}
	upath = upath[toStrip:]
	upath = "/" + upath

	f, err := page.Static.Open(path.Clean(upath))
	if err != nil {
		web.CtxLogger(r.Context()).WithError(err).WithField("sfile", path.Clean(upath)).WithField("nonclean", upath).Error("Failed serving file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stat, _ := f.Stat()

	http.ServeContent(w, r, page.Name, stat.ModTime(), f)
}
