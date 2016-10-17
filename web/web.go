package web

import (
	"crypto/tls"
	log "github.com/Sirupsen/logrus"
	"github.com/golang/crypto/acme/autocert"
	"github.com/jonas747/yagpdb/common"
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
	Templates.Funcs(template.FuncMap{
		"dict":      dictionary,
		"mTemplate": mTemplate,
		"in":        in,
	})
	log.Info("Starting yagpdb web server")

	InitOauth()
	mux := setupRoutes()

	log.Info("Running webservers")
	runServers(mux)
}

func runServers(mainMuxer *goji.Mux) {
	// launch the redir server
	go func() {
		err := http.ListenAndServe(":5000", http.HandlerFunc(httpsRedirHandler))
		if err != nil {
			log.Error("Failed http ListenAndServe:", err)
		}
	}()

	cache := autocert.DirCache("cert")

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(common.Conf.Host),
		Cache:      cache,
	}

	tlsServer := &http.Server{
		Addr:    ":5001",
		Handler: mainMuxer,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	err := tlsServer.ListenAndServeTLS("", "")
	if err != nil {
		log.Error("Failed https ListenAndServeTLS:", err)
	}
}

func setupRoutes() *goji.Mux {
	requestLogger := &lumberjack.Logger{
		Filename: "access.log",
		MaxSize:  10,
	}

	mux := goji.NewMux()
	RootMux = mux
	mux.UseC(RequestLogger(requestLogger))

	// Setup fileserver
	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))

	// General middleware
	mux.UseC(MiscMiddleware)
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

func httpsRedirHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
}
