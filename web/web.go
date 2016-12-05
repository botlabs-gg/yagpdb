package web

import (
	"crypto/tls"
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/golang/crypto/acme/autocert"
	"github.com/jonas747/yagpdb/common"
	"github.com/natefinch/lumberjack"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strings"
)

var (
	// Core template files
	Templates *template.Template

	Debug              = true // Turns on debug mode
	ListenAddressHTTP  = ":5000"
	ListenAddressHTTPS = ":5001"

	LogRequestTimestamps bool

	RootMux         *goji.Mux
	CPMux           *goji.Mux
	ServerPublicMux *goji.Mux

	properAddresses bool
)

func init() {
	Templates = template.New("")
	Templates = Templates.Funcs(template.FuncMap{
		"dict":       dictionary,
		"mTemplate":  mTemplate,
		"in":         in,
		"adjective":  common.RandomAdjective,
		"title":      strings.Title,
		"hasPerm":    hasPerm,
		"formatTime": prettyTime,
	})
	Templates = template.Must(Templates.ParseFiles("templates/index.html", "templates/cp_main.html", "templates/cp_nav.html", "templates/cp_selectserver.html", "templates/cp_logs.html"))

	flag.BoolVar(&properAddresses, "pa", false, "Sets the listen addresses to 80 and 443")
}

func Run() {

	if properAddresses {
		ListenAddressHTTP = ":80"
		ListenAddressHTTPS = ":443"
	}

	log.Info("Starting yagpdb web server http:", ListenAddressHTTP, ", and https:", ListenAddressHTTPS)

	InitOauth()
	mux := setupRoutes()

	log.Info("Running webservers")
	runServers(mux)
}

func runServers(mainMuxer *goji.Mux) {
	// launch the redir server
	go func() {
		err := http.ListenAndServe(ListenAddressHTTP, http.HandlerFunc(httpsRedirHandler))
		if err != nil {
			log.Error("Failed http ListenAndServe:", err)
		}
	}()

	cache := autocert.DirCache("cert")

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(common.Conf.Host),
		Email:      common.Conf.Email,
		Cache:      cache,
	}

	tlsServer := &http.Server{
		Addr:    ListenAddressHTTPS,
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

	// The public muxer, for public server stuff like stats and logs
	serverPublicMux := goji.SubMux()
	serverPublicMux.UseC(ActiveServerMW)

	mux.HandleC(pat.Get("/public/:server"), serverPublicMux)
	mux.HandleC(pat.Get("/public/:server/*"), serverPublicMux)
	ServerPublicMux = serverPublicMux

	// Control panel muxer, requires a session
	// cpMuxer := goji.NewMux()
	// cpMuxer.UseC(RequireSessionMiddleware)

	// mux.HandleC(pat.Get("/cp/*"), cpMuxer)
	// mux.HandleC(pat.Get("/cp"), cpMuxer)
	// mux.HandleC(pat.Post("/cp/*"), cpMuxer)
	// mux.HandleC(pat.Post("/cp"), cpMuxer)

	// Server selection has it's own handler
	mux.HandleC(pat.Get("/cp"), RenderHandler(nil, "cp_selectserver"))
	mux.HandleC(pat.Get("/cp/"), RenderHandler(nil, "cp_selectserver"))

	// Server control panel, requires you to be an admin for the server (owner or have server management role)
	serverCpMuxer := goji.SubMux()
	serverCpMuxer.UseC(RequireSessionMiddleware)
	serverCpMuxer.UseC(ActiveServerMW)
	serverCpMuxer.UseC(RequireServerAdminMiddleware)

	mux.HandleC(pat.New("/cp/:server"), serverCpMuxer)
	mux.HandleC(pat.New("/cp/:server/*"), serverCpMuxer)

	serverCpMuxer.HandleC(pat.Get("/cplogs"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	serverCpMuxer.HandleC(pat.Get("/cplogs/"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	CPMux = serverCpMuxer

	for _, plugin := range Plugins {
		plugin.InitWeb()
		log.Info("Initialized web plugin:", plugin.Name())
	}

	return mux
}

func httpsRedirHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
}
