package web

import (
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/yagpdb/common"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"log"
	"net/http"
)

var (
	// General configuration
	Config *common.Config

	RedisPool *pool.Pool

	// Core template files
	Templates = template.Must(template.ParseFiles("templates/index.html", "templates/cp_main.html", "templates/cp_nav.html", "templates/cp_selectserver.html", "templates/cp_logs.html"))

	Debug         = true // Turns on debug mode
	ListenAddress = ":5000"

	LogRequestTimestamps bool

	RootMux *goji.Mux
	CPMux   *goji.Mux
)

func Run() {

	Config = common.Conf
	RedisPool = common.RedisPool

	log.Println("Starting yagpdb web server")

	var err error

	InitOauth()
	mux := setupRoutes()

	log.Println("Running webserver!")

	err = http.ListenAndServe(ListenAddress, mux)
	if err != nil {
		log.Println("Error running webserver", err)
	}
}

func setupRoutes() *goji.Mux {
	mux := goji.NewMux()
	RootMux = mux
	// Setup fileserver
	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))

	// General middleware
	mux.UseC(RequestLoggerMiddleware)
	mux.UseC(RedisMiddleware)
	mux.UseC(BaseTemplateDataMiddleware)
	mux.UseC(SessionMiddleware)
	mux.UseC(UserInfoMiddleware)

	// General handlers
	mux.HandleC(pat.Get("/"), RenderHandler(IndexHandler, "index"))
	mux.HandleFuncC(pat.Get("/login"), HandleLogin)
	mux.HandleFuncC(pat.Get("/confirm_login"), HandleConfirmLogin)
	mux.HandleFuncC(pat.Get("/logout"), HandleLogout)

	// Control panel muxer, requires a session
	cpMuxer := goji.NewMux()
	cpMuxer.UseC(RequireSessionMiddleware)

	mux.HandleC(pat.Get("/cp/*"), cpMuxer)
	mux.HandleC(pat.Get("/cp"), cpMuxer)
	mux.HandleC(pat.Post("/cp/*"), cpMuxer)
	mux.HandleC(pat.Post("/cp"), cpMuxer)

	// Server selection has it's own handler
	cpMuxer.HandleC(pat.Get("/cp"), RenderHandler(nil, "cp_selectserver"))
	cpMuxer.HandleC(pat.Get("/cp/"), RenderHandler(nil, "cp_selectserver"))

	// Server control panel, requires you to be an admin for the server (owner or have server management role)
	serverCpMuxer := goji.SubMux()
	serverCpMuxer.UseC(RequireServerAdminMiddleware)

	cpMuxer.HandleC(pat.New("/cp/:server"), serverCpMuxer)
	cpMuxer.HandleC(pat.New("/cp/:server/*"), serverCpMuxer)

	serverCpMuxer.HandleFuncC(pat.Get("/cplogs"), HandleCPLogs)
	serverCpMuxer.HandleFuncC(pat.Get("/cplogs/"), HandleCPLogs)
	CPMux = serverCpMuxer

	for _, plugin := range plugins {
		plugin.InitWeb()
		log.Println("Initialized web plugin", plugin.Name())
	}

	return mux
}
