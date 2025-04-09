package web

import (
	"crypto/tls"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/patreon"
	yagtmpl "github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/frontend"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/web/discordblog"
	"github.com/natefinch/lumberjack"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/crypto/acme/autocert"
)

var (
	// Core template files
	Templates *template.Template

	Debug              = true // Turns on debug mode
	ListenAddressHTTP  string
	ListenAddressHTTPS string

	// Muxers
	RootMux            *goji.Mux
	CPMux              *goji.Mux
	ServerPublicMux    *goji.Mux
	ServerPublicAPIMux *goji.Mux

	confListenAddressHTTP  = config.RegisterOption("yagpdb.web.http_address", "Port to listen for HTTP requests on. Overriden by the -pa flag", 5000)
	confListenAddressHTTPS = config.RegisterOption("yagpdb.web.https_address", "Port to listen for HTTPS requests on. Overriden by the -pa flag", 5001)

	properAddresses bool

	https    bool
	exthttps bool

	acceptingRequests *int32

	globalTemplateData = TemplateData(make(map[string]interface{}))

	StartedAt = time.Now()

	CurrentAd *Advertisement

	logger = common.GetFixedPrefixLogger("web")

	confAnnouncementsChannel       = config.RegisterOption("yagpdb.announcements_channel", "Channel to pull announcements from and display on the control panel homepage", 0)
	confReverseProxyClientIPHeader = config.RegisterOption("yagpdb.web.reverse_proxy_client_ip_header", "If were behind a reverse proxy, this is the header field with the real ip that the proxy passes along", "")

	confAdPath       = config.RegisterOption("yagpdb.ad.img_path", "The ad image ", "")
	confAdLinkurl    = config.RegisterOption("yagpdb.ad.link", "Link to follow when clicking on the ad", "")
	confAdWidth      = config.RegisterOption("yagpdb.ad.w", "Ad width", 0)
	confAdHeight     = config.RegisterOption("yagpdb.ad.h", "Ad Height", 0)
	ConfAdVideos     = config.RegisterOption("yagpdb.ad.video_paths", "Comma seperated list of video paths in different formats", "")
	confDemoServerID = config.RegisterOption("yagpdb.web.demo_server_id", "Server ID for live demo links", 0)

	ConfAdsTxt = config.RegisterOption("yagpdb.ads.ads_txt", "Path to the ads.txt file for monetization using ad networks", "")

	confDisableRequestLogging = config.RegisterOption("yagpdb.disable_request_logging", "Disable logging of http requests to web server", false)

	// can be overriden by plugins
	// main prurpose is to plug in a onboarding process through a properietary plugin
	SelectServerHomePageHandler http.Handler = RenderHandler(HandleSelectServer, "cp_selectserver")
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
		"checkbox":         tmplCheckbox,
		"roleOptions":      tmplRoleDropdown,
		"roleOptionsMulti": tmplRoleDropdownMulti,

		"textChannelOptions": tmplChannelOpts([]discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews, discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildForum,
			discordgo.ChannelTypeGuildStageVoice}),
		"textChannelOptionsMulti": tmplChannelOptsMulti([]discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews, discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildForum,
			discordgo.ChannelTypeGuildStageVoice}),

		"voiceChannelOptions":      tmplChannelOpts([]discordgo.ChannelType{discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildStageVoice}),
		"voiceChannelOptionsMulti": tmplChannelOptsMulti([]discordgo.ChannelType{discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildStageVoice}),

		"catChannelOptions":      tmplChannelOpts([]discordgo.ChannelType{discordgo.ChannelTypeGuildCategory}),
		"catChannelOptionsMulti": tmplChannelOptsMulti([]discordgo.ChannelType{discordgo.ChannelTypeGuildCategory}),
	})

	Templates = Templates.Funcs(yagtmpl.StandardFuncMap)

	flag.BoolVar(&properAddresses, "pa", false, "Sets the listen addresses to 80 and 443")
	flag.BoolVar(&https, "https", true, "Serve web on HTTPS. Only disable when using an HTTPS reverse proxy.")
	flag.BoolVar(&exthttps, "exthttps", false, "Set if the website uses external https (through reverse proxy) but should only listen on http.")
}

func loadTemplates() {

	coreTemplates := []string{
		"templates/index.html", "templates/cp_main.html",
		"templates/cp_nav.html", "templates/cp_selectserver.html", "templates/cp_logs.html",
		"templates/status.html", "templates/cp_server_home.html", "templates/cp_core_settings.html",
	}

	for _, v := range coreTemplates {
		loadCoreHTMLTemplate(v)
	}
}

func BaseURL() string {
	if https || exthttps {
		return "https://" + common.ConfHost.GetString()
	}

	return "http://" + common.ConfHost.GetString()
}

func ManageServerURL(guildID int64) string {
	return fmt.Sprintf("%s/manage/%d", BaseURL(), guildID)
}

func Run() {
	common.ServiceTracker.RegisterService(common.ServiceTypeFrontend, "Webserver", "", nil)

	common.RegisterPlugin(&ControlPanelPlugin{})

	loadTemplates()

	AddGlobalTemplateData("ClientID", common.ConfClientID.GetString())
	AddGlobalTemplateData("Host", common.ConfHost.GetString())
	AddGlobalTemplateData("Version", common.VERSION)
	AddGlobalTemplateData("Testing", common.Testing)

	if properAddresses {
		ListenAddressHTTP = ":80"
		ListenAddressHTTPS = ":443"
	} else {
		ListenAddressHTTP = ":" + confListenAddressHTTP.GetString()
		ListenAddressHTTPS = ":" + confListenAddressHTTPS.GetString()
	}

	patreon.Run()

	InitOauth()
	mux := setupRoutes()

	// Start monitoring the bot
	go pollCommandsRan()

	blogChannel := confAnnouncementsChannel.GetInt()
	if blogChannel != 0 {
		go discordblog.RunPoller(common.BotSession, int64(blogChannel), time.Minute)
	}

	loadAd()

	logger.Info("Running webservers")
	runServers(mux)
}

func loadAd() {
	path := confAdPath.GetString()
	linkurl := confAdLinkurl.GetString()

	CurrentAd = &Advertisement{
		Path:    template.URL(path),
		LinkURL: template.URL(linkurl),
		Width:   confAdWidth.GetInt(),
		Height:  confAdHeight.GetInt(),
	}

	videos := strings.Split(ConfAdVideos.GetString(), ",")
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
		logger.Info("Starting yagpdb web server http:", ListenAddressHTTP)

		server := &http.Server{
			Addr:        ListenAddressHTTP,
			Handler:     mainMuxer,
			IdleTimeout: time.Minute,
		}

		err := server.ListenAndServe()
		if err != nil {
			logger.Error("Failed http ListenAndServe:", err)
		}
	} else {
		logger.Info("Starting yagpdb web server http:", ListenAddressHTTP, ", and https:", ListenAddressHTTPS)

		cache := autocert.DirCache("cert")

		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(common.ConfHost.GetString(), "www."+common.ConfHost.GetString()),
			Email:      common.ConfEmail.GetString(),
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
				logger.Error("Failed http ListenAndServe:", err)
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
			logger.Error("Failed https ListenAndServeTLS:", err)
		}
	}
}

func setupRoutes() *goji.Mux {

	// setup the root routes and middlewares
	setupRootMux()

	// Guild specific public routes, does not require admin or being logged in at all
	serverPublicMux := goji.SubMux()
	serverPublicMux.Use(ActiveServerMW)
	serverPublicMux.Use(RequireActiveServer)
	serverPublicMux.Use(LoadCoreConfigMiddleware)
	serverPublicMux.Use(SetGuildMemberMiddleware)

	RootMux.Handle(pat.New("/public/:server"), serverPublicMux)
	RootMux.Handle(pat.New("/public/:server/*"), serverPublicMux)
	ServerPublicMux = serverPublicMux

	// same as above but for API stuff
	ServerPublicAPIMux = goji.SubMux()
	ServerPublicAPIMux.Use(ActiveServerMW)
	ServerPublicAPIMux.Use(RequireActiveServer)
	ServerPublicAPIMux.Use(LoadCoreConfigMiddleware)
	ServerPublicAPIMux.Use(SetGuildMemberMiddleware)

	RootMux.Handle(pat.Get("/api/:server"), ServerPublicAPIMux)
	RootMux.Handle(pat.Get("/api/:server/*"), ServerPublicAPIMux)

	ServerPublicAPIMux.Handle(pat.Get("/channelperms/:channel"), RequireActiveServer(APIHandler(HandleChannelPermissions)))
	ServerPublicAPIMux.Handle(pat.Get("/status.json"), RequireActiveServer(APIHandler(HandleGuildStatusJSON)))

	// Server selection has its own handler
	RootMux.Handle(pat.Get("/manage"), SelectServerHomePageHandler)
	RootMux.Handle(pat.Get("/manage/"), SelectServerHomePageHandler)
	RootMux.Handle(pat.Get("/status"), ControllerHandler(HandleStatusHTML, "cp_status"))
	RootMux.Handle(pat.Get("/status/"), ControllerHandler(HandleStatusHTML, "cp_status"))
	RootMux.Handle(pat.Get("/status.json"), APIHandler(HandleStatusJSON))
	RootMux.Handle(pat.Post("/shard/:shard/reconnect"), ControllerHandler(HandleReconnectShard, "cp_status"))
	RootMux.Handle(pat.Post("/shard/:shard/reconnect/"), ControllerHandler(HandleReconnectShard, "cp_status"))

	RootMux.HandleFunc(pat.Get("/cp"), legacyCPRedirHandler)
	RootMux.HandleFunc(pat.Get("/cp/*"), legacyCPRedirHandler)

	// Server control panel, requires you to be an admin for the server (owner or have server management role)
	CPMux = goji.SubMux()
	CPMux.Use(ActiveServerMW)
	CPMux.Use(RequireActiveServer)
	CPMux.Use(LoadCoreConfigMiddleware)
	CPMux.Use(SetGuildMemberMiddleware)
	CPMux.Use(RequireServerAdminMiddleware)

	RootMux.Handle(pat.New("/manage/:server"), CPMux)
	RootMux.Handle(pat.New("/manage/:server/*"), CPMux)

	CPMux.Handle(pat.Get("/cplogs"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	CPMux.Handle(pat.Get("/cplogs/"), RenderHandler(HandleCPLogs, "cp_action_logs"))
	CPMux.Handle(pat.Get("/home"), ControllerHandler(HandleServerHome, "cp_server_home"))
	CPMux.Handle(pat.Get("/home/"), ControllerHandler(HandleServerHome, "cp_server_home"))

	coreSettingsHandler := RenderHandler(nil, "cp_core_settings")

	CPMux.Handle(pat.Get("/core/"), coreSettingsHandler)
	CPMux.Handle(pat.Get("/core"), coreSettingsHandler)
	CPMux.Handle(pat.Post("/core"), ControllerPostHandler(HandlePostCoreSettings, coreSettingsHandler, CoreConfigPostForm{}))

	RootMux.Handle(pat.Get("/guild_selection"), RequireSessionMiddleware(ControllerHandler(HandleGetManagedGuilds, "cp_guild_selection")))
	CPMux.Handle(pat.Get("/guild_selection"), RequireSessionMiddleware(ControllerHandler(HandleGetManagedGuilds, "cp_guild_selection")))

	// Set up the routes for the per server home widgets
	for _, p := range common.Plugins {
		if cast, ok := p.(PluginWithServerHomeWidget); ok {
			handler := GuildScopeCacheMW(p, ControllerHandler(cast.LoadServerHomeWidget, "cp_server_home_widget"))

			if mwares, ok2 := p.(PluginWithServerHomeWidgetMiddlewares); ok2 {
				handler = mwares.ServerHomeWidgetApplyMiddlewares(handler)
			}

			CPMux.Handle(pat.Get("/homewidgets/"+p.PluginInfo().SysName), handler)
		}
	}

	AddSidebarItem(SidebarCategoryCore, &SidebarItem{
		Name: "Control panel access",
		URL:  "core",
		Icon: "fas fa-cog",
	})

	AddSidebarItem(SidebarCategoryCore, &SidebarItem{
		Name: "Control panel logs",
		URL:  "cplogs",
		Icon: "fas fa-database",
	})

	for _, plugin := range common.Plugins {
		if webPlugin, ok := plugin.(Plugin); ok {
			webPlugin.InitWeb()
			logger.Info("Initialized web plugin:", plugin.PluginInfo().Name)
		}
	}

	return RootMux
}

var StaticFilesFS fs.FS = frontend.StaticFiles

func setupRootMux() {
	mux := goji.NewMux()
	RootMux = mux

	if !confDisableRequestLogging.GetBool() {
		requestLogger := &lumberjack.Logger{
			Filename: "access.log",
			MaxSize:  10,
		}

		mux.Use(RequestLogger(requestLogger))
	}

	// Setup fileserver
	mux.Handle(pat.Get("/static/*"), http.FileServer(http.FS(StaticFilesFS)))
	mux.Handle(pat.Get("/robots.txt"), http.HandlerFunc(handleRobotsTXT))
	mux.Handle(pat.Get("/ads.txt"), http.HandlerFunc(handleAdsTXT))

	// General middleware
	mux.Use(SkipStaticMW(gziphandler.GzipHandler, ".css", ".js", ".map"))
	mux.Use(SkipStaticMW(MiscMiddleware))
	mux.Use(SkipStaticMW(BaseTemplateDataMiddleware))
	mux.Use(SkipStaticMW(SessionMiddleware))
	mux.Use(SkipStaticMW(UserInfoMiddleware))
	mux.Use(SkipStaticMW(CSRFProtectionMW))
	mux.Use(addPromCountMW)

	// General handlers
	mux.Handle(pat.Get("/"), ControllerHandler(HandleLandingPage, "index"))
	mux.HandleFunc(pat.Get("/login"), HandleLogin)
	mux.HandleFunc(pat.Get("/confirm_login"), HandleConfirmLogin)
	mux.HandleFunc(pat.Get("/logout"), HandleLogout)
}

func httpsRedirHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
}

func AddGlobalTemplateData(key string, data interface{}) {
	globalTemplateData[key] = data
}

func legacyCPRedirHandler(w http.ResponseWriter, r *http.Request) {
	logger.Println("Hit cp path: ", r.RequestURI)
	trimmed := strings.TrimPrefix(r.RequestURI, "/cp")
	http.Redirect(w, r, "/manage"+trimmed, http.StatusMovedPermanently)
}

func AddHTMLTemplate(name, contents string) {
	Templates = Templates.New(name)
	Templates = template.Must(Templates.Parse(contents))
}

func loadCoreHTMLTemplate(path string) {
	contents, err := frontend.CoreTemplates.ReadFile(path)
	if err != nil {
		panic(err)
	}
	Templates = Templates.New(path)
	Templates = template.Must(Templates.Parse(string(contents)))
}

const (
	SidebarCategoryTopLevel       = "Top"
	SidebarCategoryFeeds          = "Feeds"
	SidebarCategoryTools          = "Tools"
	SidebarCategoryFun            = "Fun"
	SidebarCategoryCore           = "Core"
	SidebarCategoryCustomCommands = "CustomCommands"
	SidebarCategoryModeration     = "Moderation"
)

type SidebarItem struct {
	Name            string
	URL             string
	Icon            string
	CustomIconImage string
	New             bool
	External        bool
}

var sideBarItems = make(map[string][]*SidebarItem)

func AddSidebarItem(category string, sItem *SidebarItem) {
	sideBarItems[category] = append(sideBarItems[category], sItem)
}
