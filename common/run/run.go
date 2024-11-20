package run

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/common/sentryhook"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
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
	FlagGenCmdDocs    bool
	flagGenConfigDocs bool

	flagLogAppName string

	flagNodeID string

	flagVersion bool
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
	flag.BoolVar(&FlagGenCmdDocs, "gencmddocs", false, "Generate command docs and exit")
	flag.BoolVar(&flagGenConfigDocs, "genconfigdocs", false, "Generate config docs and exit")

	flag.BoolVar(&flagLogTimestamp, "ts", false, "Set to include timestamps in log")

	flag.StringVar(&flagNodeID, "nodeid", "", "The id of this node, used when running with a sharding orchestrator")
	flag.BoolVar(&flagVersion, "version", false, "Print the version and exit")
}

func Init() {
	if !flag.Parsed() {
		flag.Parse()
	}

	if flagVersion {
		fmt.Println(common.VERSION)
		os.Exit(0)
	}

	common.NodeID = flagNodeID

	common.AddLogHook(common.ContextHook{})

	common.SetLogFormatter(&log.TextFormatter{
		DisableTimestamp: !common.Testing,
		ForceColors:      common.Testing,
		SortingFunc:      logrusSortingFunc,
	})

	if flagSysLog {
		AddSyslogHooks()
	}

	if !flagRunBot && !flagRunWeb && flagRunFeeds == "" && !flagRunEverything && !flagDryRun && !flagRunBWC && !flagGenConfigDocs {
		log.Error("Didnt specify what to run, see -h for more info")
		os.Exit(1)
	}

	log.Info("Starting YAGPDB version " + common.VERSION)

	err := common.CoreInit(true)
	if err != nil {
		log.WithError(err).Fatal("Failed running core init ")
	}

	if confSentryDSN.GetString() != "" {
		addSentryHook()
	}

	err = common.Init()
	if err != nil {
		log.WithError(err).Fatal("Failed intializing")
	}

	log.Info("Starting plugins")
}

func Run() {
	if flagDryRun {
		log.Println("This is a dry run, exiting")
		return
	}

	if flagRunWeb {
		// web should handle all events
		pubsub.FilterFunc = func(guildID int64) bool {
			return true
		}
	}

	if flagRunBot || flagRunEverything {
		bot.Enabled = true
	}

	commands.InitCommands()

	if FlagGenCmdDocs {
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

	if flagRunBot || flagRunEverything || flagRunBWC {
		mqueue.RegisterPlugin()
	}

	if flagRunBot || flagRunEverything {
		botrest.RegisterPlugin()
		bot.Run(flagNodeID)
	}

	if flagRunFeeds != "" || flagRunEverything {
		var runFeeds []string
		if !flagRunEverything {
			runFeeds = strings.Split(flagRunFeeds, ",")
		}
		go feeds.Run(runFeeds)
	}

	if flagRunBWC || flagRunEverything {
		go backgroundworkers.RunWorkers()
	}

	go pubsub.PollEvents()

	common.RunCommonRunPlugins()

	common.SetShutdownFunc(shutdown)
	listenSignal()
}

// Gracefull shutdown
// Why we sleep before we stop? just to be on the safe side in case there's some stuff that's not fully done yet
// running in seperate untracked goroutines
func listenSignal() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	common.Shutdown()
}

func shutdown() {
	log.Info("SHUTTING DOWN... ")

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

func addSentryHook() {
	err := sentry.Init(sentry.ClientOptions{
		// Either set your DSN here or set the SENTRY_DSN environment variable.
		Dsn: confSentryDSN.GetString(),
		// Enable printing of SDK debug messages.
		// Useful when getting started or trying to figure something out.
		Debug: false,
	})

	if err == nil {
		sentry.ConfigureScope(func(s *sentry.Scope) {
			if flagNodeID != "" {
				s.SetTag("node_id", flagNodeID)
			}
		})

		hook := &sentryhook.Hook{}
		common.AddLogHook(hook)
		log.Info("Added Sentry Hook")
	} else {
		log.WithError(err).Error("Failed adding sentry hook")
	}
}

var logSortPriority = []string{
	"time",
	"level",
	"p",
	"msg",
	"stck",
}

func logrusSortingFunc(fields []string) {
	sort.Slice(fields, func(i, j int) bool {

		iPriority := findStringIndex(logSortPriority, fields[i])
		jPriority := findStringIndex(logSortPriority, fields[j])

		if iPriority != -1 && jPriority == -1 {
			return true
		} else if jPriority != -1 && iPriority == -1 {
			return false
		} else if iPriority == -1 && jPriority == -1 {
			return strings.Compare(fields[i], fields[j]) > 1
		}

		// both has priority
		return iPriority < jPriority
	})
}

func findStringIndex(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}

	return -1
}
