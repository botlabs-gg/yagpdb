package web

import (
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/yagpdb/common"
	"html/template"
	"log"
	"net/http"

	"goji.io"
	"goji.io/pat"
)

var (
	// General configuration
	Config *common.Config

	RedisPool *pool.Pool

	// Core template files
	Templates = template.Must(template.ParseFiles("templates/index.html", "templates/cp_main.html", "templates/cp_nav.html"))

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

	mux.UseC(RequestLoggerMiddleware)
	mux.UseC(RedisMiddleware)
	mux.UseC(SessionMiddleware)

	mux.HandleFuncC(pat.Get("/"), IndexHandler)
	mux.HandleFuncC(pat.Get("/login"), HandleLogin)
	mux.HandleFuncC(pat.Get("/confirm_login"), HandleConfirmLogin)

	cpMuxer := goji.NewMux()
	cpMuxer.UseC(RequireSessionMiddleware)

	mux.Handle(pat.Get("/cp/*"), cpMuxer)
	mux.Handle(pat.Get("/cp"), cpMuxer)

	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))
	//mux.HandleC(pat.Get("/dashboard"), RequireSessionMiddleware(goji.HandlerFunc(DashboardIndex)))

	for _, plugin := range plugins {
		plugin.Init(mux, cpMuxer)
		log.Println("Initialized", plugin.Name())
	}

	return mux
}
