package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/evalphobia/logrus_sentry"
	"github.com/jonas747/yagpdb/automod"
	"github.com/jonas747/yagpdb/safebrowsing"
	log "github.com/sirupsen/logrus"

	// Core yagpdb packages
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/jonas747/yagpdb/web"

	// Plugin imports
	"github.com/jonas747/yagpdb/automod_legacy"
	"github.com/jonas747/yagpdb/autorole"
	"github.com/jonas747/yagpdb/aylien"
	"github.com/jonas747/yagpdb/cah"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/discordlogger"
	"github.com/jonas747/yagpdb/logs"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/premium/patreonpremiumsource"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/reminders"
	"github.com/jonas747/yagpdb/reputation"
	"github.com/jonas747/yagpdb/rolecommands"
	"github.com/jonas747/yagpdb/rsvp"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/soundboard"
	"github.com/jonas747/yagpdb/stdcommands"
	"github.com/jonas747/yagpdb/streaming"
	"github.com/jonas747/yagpdb/tickets"
	"github.com/jonas747/yagpdb/timezonecompanion"
	"github.com/jonas747/yagpdb/twitter"
	"github.com/jonas747/yagpdb/verification"
	"github.com/jonas747/yagpdb/youtube"
	// External plugins
)

var (
	flagRunBot        bool
	flagRunWeb        bool
	flagRunFeeds      string
	flagRunEverything bool
	flagRunBWC        bool

	flagDryRun bool

	flagLogTimestamp bool

	flagSysLog        bool
	flagGenCmdDocs    bool
	flagGenConfigDocs bool

	flagLogAppName string

	flagNodeID string
)

var confSentryDSN = config.RegisterOption("yagpdb.sentry_dsn", "Sentry credentials for sentry logging hook", nil)

func init() {
	flag.BoolVar(&flagRunBot, "bot", false, "Set to run discord bot and bot related stuff")
	flag.BoolVar(&flagRunWeb, "web", false, "Set to run webserver")
	flag.StringVar(&flagRunFeeds, "feeds", "", "Which feeds to run, comma seperated list (currently reddit, youtube and twitter)")
	flag.BoolVar(&flagRunEverything, "all", false, "Set to everything (discord bot, webserver, backgroundworkers and all feeds)")
	flag.BoolVar(&flagDryRun, "dry", false, "Do a dryrun, initialize all plugins but don't actually start anything")
	flag.BoolVar(&flagSysLog, "syslog", false, "Set to log to syslog (only linux)")
	flag.StringVar(&flagLogAppName, "logappname", "yagpdb", "When using syslog, the application name will be set to this")
	flag.BoolVar(&flagRunBWC, "backgroundworkers", false, "Run the various background workers, atleast one process needs this")
	flag.BoolVar(&flagGenCmdDocs, "gencmddocs", false, "Generate command docs and exit")
	flag.BoolVar(&flagGenConfigDocs, "genconfigdocs", false, "Generate config docs and exit")

	flag.BoolVar(&flagLogTimestamp, "ts", false, "Set to include timestamps in log")

	flag.StringVar(&flagNodeID, "nodeid", "", "The id of this node, used when running with a sharding orchestrator")
}

func main() {
	flag.Parse()
	common.NodeID = flagNodeID

	common.AddLogHook(common.ContextHook{})

	common.SetLogFormatter(&log.TextFormatter{
		DisableTimestamp: !common.Testing,
		ForceColors:      common.Testing,
	})

	if flagSysLog {
		AddSyslogHooks()
	}

	config.Load()
	if confSentryDSN.GetString() != "" {
		hook, err := logrus_sentry.NewSentryHook(confSentryDSN.GetString(), []log.Level{
			log.PanicLevel,
			log.FatalLevel,
			log.ErrorLevel,
		})

		if err == nil {
			common.AddLogHook(hook)
			log.Info("Added Sentry Hook")
		} else {
			log.WithError(err).Error("Failed adding sentry hook")
		}
	}

	if !flagRunBot && !flagRunWeb && flagRunFeeds == "" && !flagRunEverything && !flagDryRun && !flagRunBWC && !flagGenConfigDocs {
		log.Error("Didnt specify what to run, see -h for more info")
		return
	}

	log.Info("YAGPDB is initializing...")

	err := common.Init()
	if err != nil {
		log.WithError(err).Fatal("Failed intializing")
	}

	log.Info("Initiliazing generic config store")
	configstore.InitDatabases()

	log.Info("Starting plugins")

	//BotSession.LogLevel = discordgo.LogInformational
	paginatedmessages.RegisterPlugin()

	// Setup plugins
	safebrowsing.RegisterPlugin()
	discordlogger.Register()
	commands.RegisterPlugin()
	stdcommands.RegisterPlugin()
	serverstats.RegisterPlugin()
	notifications.RegisterPlugin()
	customcommands.RegisterPlugin()
	reddit.RegisterPlugin()
	moderation.RegisterPlugin()
	reputation.RegisterPlugin()
	aylien.RegisterPlugin()
	streaming.RegisterPlugin()
	automod_legacy.RegisterPlugin()
	automod.RegisterPlugin()
	logs.RegisterPlugin()
	autorole.RegisterPlugin()
	reminders.RegisterPlugin()
	soundboard.RegisterPlugin()
	youtube.RegisterPlugin()
	rolecommands.RegisterPlugin()
	cah.RegisterPlugin()
	tickets.RegisterPlugin()
	verification.RegisterPlugin()
	premium.RegisterPlugin()
	patreonpremiumsource.RegisterPlugin()
	scheduledevents2.RegisterPlugin()
	twitter.RegisterPlugin()
	rsvp.RegisterPlugin()
	timezonecompanion.RegisterPlugin()

	if flagDryRun {
		log.Println("This is a dry run, exiting")
		return
	}

	if flagRunBot || flagRunEverything {
		bot.Enabled = true
	}

	commands.InitCommands()

	if flagGenCmdDocs {
		GenCommandsDocs()
		return
	}

	if flagGenConfigDocs {
		GenConfigDocs()
		return
	}

	if flagRunWeb || flagRunEverything {
		go web.Run()
	}

	if flagRunBot || flagRunEverything {
		mqueue.RegisterPlugin()
		botrest.RegisterPlugin()
		bot.Run(flagNodeID)
	}

	if flagRunFeeds != "" || flagRunEverything {
		go feeds.Run(strings.Split(flagRunFeeds, ","))
	}

	if flagRunBWC || flagRunEverything {
		go backgroundworkers.RunWorkers()
	}

	go pubsub.PollEvents()

	listenSignal()
}

// Gracefull shutdown
// Why we sleep before we stop? just to be on the safe side in case there's some stuff that's not fully done yet
// running in seperate untracked goroutines
func listenSignal() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	sig := <-c
	log.Info("SHUTTING DOWN... ", sig.String())

	shouldWait := false
	wg := new(sync.WaitGroup)

	if flagRunBot || flagRunEverything {

		wg.Add(1)

		go bot.Stop(wg)

		shouldWait = true
	}

	if flagRunFeeds != "" || flagRunEverything {
		feeds.Stop(wg)
		shouldWait = true
	}

	if flagRunWeb {
		web.Stop()
		// Slep for a extra second
		time.Sleep(time.Second)
	}

	if flagRunBWC {
		backgroundworkers.StopWorkers(wg)
	}

	if shouldWait {
		log.Info("Waiting for things to shut down...")
		wg.Wait()
	}

	log.Info("Sleeping for a second to allow work to finish")
	time.Sleep(time.Second)

	log.Info("Bye..")
	os.Exit(0)
}
