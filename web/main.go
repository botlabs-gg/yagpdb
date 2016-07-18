package main

import (
	"github.com/fzzy/radix/extra/pool"
	"html/template"
	"log"
	"net/http"

	"goji.io"
	"goji.io/pat"
)

var (
	config    *Config
	templates = template.Must(template.ParseFiles("templates/index.html"))
	redisPool *pool.Pool
)

func main() {
	log.Println("Starting yagpdb web server")

	var err error
	config, err = LoadConfig("config.json")
	if err != nil {
		log.Println("Failed loading config", err)
		return
	}

	redisPool, err = pool.NewPool("tcp", config.Redis, 10)
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

	mux.UseC(SessionMiddleware)
	mux.UseC(RedisMiddleware)

	mux.HandleFuncC(pat.Get("/"), index)
	mux.HandleFuncC(pat.Get("/login"), handleLogin)
	mux.HandleFuncC(pat.Get("/confirm_login"), handleConfirmLogin)

	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))

	return mux
}
