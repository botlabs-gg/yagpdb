package web

import (
	log "github.com/Sirupsen/logrus"
	"github.com/natefinch/lumberjack"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
)

var (
	// Core template files
	Templates = template.Must(template.ParseFiles("templates/index.html", "templates/cp_main.html", "templates/cp_nav.html", "templates/cp_selectserver.html", "templates/cp_logs.html"))

	Debug         = true // Turns on debug mode
	ListenAddress = ":5000"

	LogRequestTimestamps bool

	RootMux *goji.Mux
	CPMux   *goji.Mux
)

func Run() {
	log.Info("Starting yagpdb web server")
	var err error

	InitOauth()
	mux := setupRoutes()

	log.Info("Running webserver")
	err = http.ListenAndServe(ListenAddress, mux)
	if err != nil {
		log.Error("Failed ListenAndServe:", err)
	}
}

func setupRoutes() *goji.Mux {
	requestLogger := &lumberjack.Logger{
		Filename: "access_log",
		MaxSize:  10,
	}

	mux := goji.NewMux()
	RootMux = mux
	mux.UseC(RequestLogger(requestLogger))

	// Setup fileserver
	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))

	// General middleware
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

	serverCpMuxer.HandleC(pat.Get("/cplogs"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	serverCpMuxer.HandleC(pat.Get("/cplogs/"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	CPMux = serverCpMuxer

	for _, plugin := range plugins {
		plugin.InitWeb()
		log.Info("Initialized web plugin:", plugin.Name())
	}

	return mux
}
