package web

import (
	"crypto/tls"
	"flag"
	"github.com/NYTimes/gziphandler"
	"github.com/golang/crypto/acme/autocert"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/patreon"
	yagtmpl "github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/web/discordblog"
	"github.com/natefinch/lumberjack"
	log "github.com/sirupsen/logrus"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var (
	// Core template files
	Templates *template.Template

	Debug              = true // Turns on debug mode
	ListenAddressHTTP  = ":5000"
	ListenAddressHTTPS = ":5001"

	// Muxers
	RootMux           *goji.Mux
	CPMux             *goji.Mux
	ServerPublicMux   *goji.Mux
	ServerPubliAPIMux *goji.Mux

	properAddresses bool

	https bool

	acceptingRequests *int32

	globalTemplateData = TemplateData(make(map[string]interface{}))

	StartedAt = time.Now()

	CurrentAd *Advertisement
)

type Advertisement struct {
	Path       template.URL
	VideoUrls  []template.URL
	VideoTypes []string
	LinkURL    template.URL
	Width      int
	Height     int
}

func init() {
	b := int32(1)
	acceptingRequests = &b

	Templates = template.New("")
	Templates = Templates.Funcs(template.FuncMap{
		"mTemplate":        mTemplate,
		"hasPerm":          hasPerm,
		"formatTime":       prettyTime,
		"roleOptions":      tmplRoleDropdown,
		"roleOptionsMulti": tmplRoleDropdownMutli,

		"textChannelOptions":      tmplChannelOpts(discordgo.ChannelTypeGuildText, "#"),
		"textChannelOptionsMulti": tmplChannelOptsMulti(discordgo.ChannelTypeGuildText, "#"),

		"voiceChannelOptions":      tmplChannelOpts(discordgo.ChannelTypeGuildVoice, ""),
		"voiceChannelOptionsMulti": tmplChannelOptsMulti(discordgo.ChannelTypeGuildVoice, ""),

		"catChannelOptions":      tmplChannelOpts(discordgo.ChannelTypeGuildCategory, ""),
		"catChannelOptionsMulti": tmplChannelOptsMulti(discordgo.ChannelTypeGuildCategory, ""),
	})

	Templates = Templates.Funcs(yagtmpl.StandardFuncMap)

	flag.BoolVar(&properAddresses, "pa", false, "Sets the listen addresses to 80 and 443")
	flag.BoolVar(&https, "https", true, "Serve web on HTTPS. Only disable when using an HTTPS reverse proxy.")
}

func LoadTemplates() {
	Templates = template.Must(Templates.ParseFiles("templates/index.html", "templates/cp_main.html", "templates/cp_nav.html", "templates/cp_selectserver.html", "templates/cp_logs.html", "templates/status.html"))
}

func Run() {
	LoadTemplates()

	AddGlobalTemplateData("ClientID", common.Conf.ClientID)
	AddGlobalTemplateData("Host", common.Conf.Host)
	AddGlobalTemplateData("Version", common.VERSION)
	AddGlobalTemplateData("Testing", common.Testing)

	if properAddresses {
		ListenAddressHTTP = ":80"
		ListenAddressHTTPS = ":443"
	}

	InitOauth()
	mux := setupRoutes()

	// Start monitoring the bot
	go botrest.RunPinger()

	blogChannel := os.Getenv("YAGPDB_ANNOUNCEMENTS_CHANNEL")
	parsedBlogChannel, _ := strconv.ParseInt(blogChannel, 10, 64)
	if parsedBlogChannel != 0 {
		go discordblog.RunPoller(common.BotSession, parsedBlogChannel, time.Minute)
	}

	patreon.Run()

	LoadAd()

	log.Info("Running webservers")
	runServers(mux)
}

func LoadAd() {
	path := os.Getenv("YAGPDB_AD_IMG_PATH")
	linkurl := os.Getenv("YAGPDB_AD_LINK")
	width, _ := strconv.Atoi(os.Getenv("YAGPDB_AD_W"))
	height, _ := strconv.Atoi(os.Getenv("YAGPDB_AD_H"))

	CurrentAd = &Advertisement{
		Path:    template.URL(path),
		LinkURL: template.URL(linkurl),
		Width:   width,
		Height:  height,
	}

	videos := strings.Split(os.Getenv("YAGPDB_AD_VIDEO_PATHS"), ",")
	for _, v := range videos {
		if v == "" {
			continue
		}
		CurrentAd.VideoUrls = append(CurrentAd.VideoUrls, template.URL(v))

		split := strings.SplitN(v, ".", 2)
		if len(split) < 2 {
			CurrentAd.VideoTypes = append(CurrentAd.VideoTypes, "unknown")
			continue
		}

		CurrentAd.VideoTypes = append(CurrentAd.VideoTypes, "video/"+split[1])
	}
}

func Stop() {
	atomic.StoreInt32(acceptingRequests, 0)
}

func IsAcceptingRequests() bool {
	return atomic.LoadInt32(acceptingRequests) != 0
}

func runServers(mainMuxer *goji.Mux) {
	if !https {
		log.Info("Starting yagpdb web server http:", ListenAddressHTTP)

		server := &http.Server{
			Addr:        ListenAddressHTTP,
			Handler:     mainMuxer,
			IdleTimeout: time.Minute,
		}

		err := server.ListenAndServe()
		if err != nil {
			log.Error("Failed http ListenAndServe:", err)
		}
	} else {
		log.Info("Starting yagpdb web server http:", ListenAddressHTTP, ", and https:", ListenAddressHTTPS)

		cache := autocert.DirCache("cert")

		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(common.Conf.Host, "www."+common.Conf.Host),
			Email:      common.Conf.Email,
			Cache:      cache,
		}

		// launch the redir server
		go func() {
			unsafeHandler := &http.Server{
				Addr:        ListenAddressHTTP,
				Handler:     certManager.HTTPHandler(http.HandlerFunc(httpsRedirHandler)),
				IdleTimeout: time.Minute,
			}

			err := unsafeHandler.ListenAndServe()
			if err != nil {
				log.Error("Failed http ListenAndServe:", err)
			}
		}()

		tlsServer := &http.Server{
			Addr:        ListenAddressHTTPS,
			Handler:     mainMuxer,
			IdleTimeout: time.Minute,
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
			},
		}

		err := tlsServer.ListenAndServeTLS("", "")
		if err != nil {
			log.Error("Failed https ListenAndServeTLS:", err)
		}
	}
}

func setupRoutes() *goji.Mux {

	mux := goji.NewMux()
	RootMux = mux

	if os.Getenv("YAGPDB_DISABLE_REQUEST_LOGGING") == "" {
		requestLogger := &lumberjack.Logger{
			Filename: "access.log",
			MaxSize:  10,
		}

		mux.Use(RequestLogger(requestLogger))
	}

	// Setup fileserver
	mux.Handle(pat.Get("/static/*"), http.FileServer(http.Dir(".")))

	// General middleware
	mux.Use(gziphandler.GzipHandler)
	mux.Use(MiscMiddleware)
	mux.Use(BaseTemplateDataMiddleware)
	mux.Use(SessionMiddleware)
	mux.Use(UserInfoMiddleware)

	// General handlers
	mux.Handle(pat.Get("/"), ControllerHandler(HandleLandingPage, "index"))
	mux.HandleFunc(pat.Get("/login"), HandleLogin)
	mux.HandleFunc(pat.Get("/confirm_login"), HandleConfirmLogin)
	mux.HandleFunc(pat.Get("/logout"), HandleLogout)

	// The public muxer, for public server stuff like stats and logs
	serverPublicMux := goji.SubMux()
	serverPublicMux.Use(ActiveServerMW)
	mux.Handle(pat.Get("/public/:server"), serverPublicMux)
	mux.Handle(pat.Get("/public/:server/*"), serverPublicMux)
	ServerPublicMux = serverPublicMux

	ServerPubliAPIMux = goji.SubMux()
	ServerPubliAPIMux.Use(ActiveServerMW)
	mux.Handle(pat.Get("/api/:server"), ServerPubliAPIMux)
	mux.Handle(pat.Get("/api/:server/*"), ServerPubliAPIMux)

	ServerPubliAPIMux.Handle(pat.Get("/channelperms/:channel"), RequireActiveServer(APIHandler(HandleChanenlPermissions)))

	// Server selection has it's own handler
	mux.Handle(pat.Get("/manage"), RenderHandler(HandleSelectServer, "cp_selectserver"))
	mux.Handle(pat.Get("/manage/"), RenderHandler(HandleSelectServer, "cp_selectserver"))
	mux.Handle(pat.Get("/status"), ControllerHandler(HandleStatus, "cp_status"))
	mux.Handle(pat.Get("/status/"), ControllerHandler(HandleStatus, "cp_status"))
	mux.Handle(pat.Post("/shard/:shard/reconnect"), ControllerHandler(HandleReconnectShard, "cp_status"))
	mux.Handle(pat.Post("/shard/:shard/reconnect/"), ControllerHandler(HandleReconnectShard, "cp_status"))

	mux.HandleFunc(pat.Get("/cp"), legacyCPRedirHandler)
	mux.HandleFunc(pat.Get("/cp/*"), legacyCPRedirHandler)

	// Server control panel, requires you to be an admin for the server (owner or have server management role)
	serverCpMuxer := goji.SubMux()
	serverCpMuxer.Use(RequireSessionMiddleware)
	serverCpMuxer.Use(ActiveServerMW)
	serverCpMuxer.Use(RequireServerAdminMiddleware)

	mux.Handle(pat.New("/manage/:server"), serverCpMuxer)
	mux.Handle(pat.New("/manage/:server/*"), serverCpMuxer)

	serverCpMuxer.Handle(pat.Get("/cplogs"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	serverCpMuxer.Handle(pat.Get("/cplogs/"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	CPMux = serverCpMuxer

	for _, plugin := range common.Plugins {
		if webPlugin, ok := plugin.(Plugin); ok {
			webPlugin.InitWeb()
			log.Info("Initialized web plugin:", plugin.Name())
		}
	}

	return mux
}

func httpsRedirHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
}

func AddGlobalTemplateData(key string, data interface{}) {
	globalTemplateData[key] = data
}

func legacyCPRedirHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Hit cp path: ", r.RequestURI)
	trimmed := strings.TrimPrefix(r.RequestURI, "/cp")
	http.Redirect(w, r, "/manage"+trimmed, http.StatusMovedPermanently)
}
