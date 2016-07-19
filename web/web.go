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
	Templates = template.Must(template.ParseFiles("templates/index.html", "templates/cp_main.html", "templates/cp_nav.html", "templates/cp_selectserver.html"))

	Debug = true // Turns on debug mode
)

func Run() {
	log.Println("Starting yagpdb web server")

	var err error
	RedisPool, err = pool.NewPool("tcp", Config.Redis, 10)
	if err != nil {
		log.Println("Failed initializing redis pool")
		return
	}

	InitOauth()
	mux := setupRoutes()

	log.Println("Running webserver!")

	err = http.ListenAndServe(":5000", mux)
	if err != nil {
		log.Println("Error running webserver", err)
	}
}

func setupRoutes() *goji.Mux {
	mux := goji.NewMux()

	// Setup fileserver
	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))

	// General middleware
	mux.UseC(RequestLoggerMiddleware)
	mux.UseC(RedisMiddleware)
	mux.UseC(SessionMiddleware)
	mux.UseC(UserInfoMiddleware)

	// General handlers
	mux.HandleFuncC(pat.Get("/"), IndexHandler)
	mux.HandleFuncC(pat.Get("/login"), HandleLogin)
	mux.HandleFuncC(pat.Get("/confirm_login"), HandleConfirmLogin)
	mux.HandleFuncC(pat.Get("/logout"), HandleLogout)

	// Control panel muxer, requires a session
	cpMuxer := goji.NewMux()
	mux.HandleC(pat.Get("/cp/*"), cpMuxer)
	mux.HandleC(pat.Get("/cp"), cpMuxer)
	cpMuxer.UseC(RequireSessionMiddleware)

	// Server selection has it's own handler
	cpMuxer.HandleFuncC(pat.Get("/cp"), HandleSelectServer)
	cpMuxer.HandleFuncC(pat.Get("/cp/"), HandleSelectServer)

	// Server control panel, requires you to be an admin for the server (owner or have server management role)
	serverCpMuxer := goji.NewMux()
	cpMuxer.HandleC(pat.Get("/cp/:server"), serverCpMuxer)
	cpMuxer.HandleC(pat.Get("/cp/:server/*"), serverCpMuxer)
	serverCpMuxer.UseC(RequireServerAdminMiddleware)

	for _, plugin := range plugins {
		plugin.InitWeb(mux, serverCpMuxer)
		log.Println("Initialized web plugin", plugin.Name())
	}

	return mux
}
