package main

import (
	"flag"
	"github.com/evalphobia/logrus_sentry"
	"github.com/mediocregopher/radix.v2/redis"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	// Core yagpdb packages
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/jonas747/yagpdb/web"
	// Plugin imports
	"github.com/jonas747/yagpdb/automod"
	"github.com/jonas747/yagpdb/autorole"
	"github.com/jonas747/yagpdb/aylien"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/discordlogger"
	"github.com/jonas747/yagpdb/docs"
	"github.com/jonas747/yagpdb/logs"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/reminders"
	"github.com/jonas747/yagpdb/reputation"
	"github.com/jonas747/yagpdb/rolecommands"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/soundboard"
	"github.com/jonas747/yagpdb/stdcommands"
	"github.com/jonas747/yagpdb/streaming"
	"github.com/jonas747/yagpdb/youtube"
)

var (
	flagRunBot        bool
	flagRunWeb        bool
	flagRunFeeds      string
	flagRunEverything bool
	flagDryRun        bool

	flagAction string

	flagLogTimestamp bool
)

func init() {
	flag.BoolVar(&flagRunBot, "bot", false, "Set to run discord bot and bot related stuff")
	flag.BoolVar(&flagRunWeb, "web", false, "Set to run webserver")
	flag.StringVar(&flagRunFeeds, "feeds", "", "Which feeds to run, comma seperated list (currently reddit and youtube)")
	flag.BoolVar(&flagRunEverything, "all", false, "Set to everything (discord bot, webserver and reddit bot)")
	flag.BoolVar(&flagDryRun, "dry", false, "Do a dryrun, initialize all plugins but don't actually start anything")

	flag.BoolVar(&flagLogTimestamp, "ts", false, "Set to include timestamps in log")
	flag.StringVar(&flagAction, "a", "", "Run a action and exit, available actions: connected, migrate")
}

func main() {
	flag.Parse()

	log.AddHook(common.ContextHook{})

	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: !common.Testing,
		ForceColors:      true,
	})

	if os.Getenv("YAGPDB_SENTRY_DSN") != "" {
		hook, err := logrus_sentry.NewSentryHook(os.Getenv("YAGPDB_SENTRY_DSN"), []log.Level{
			log.PanicLevel,
			log.FatalLevel,
			log.ErrorLevel,
		})

		if err == nil {
			log.AddHook(hook)
			log.Info("Added Sentry Hook")
		} else {
			log.WithError(err).Error("Failed adding sentry hook")
		}
	}

	// log.SetOutput(ansicolor.NewAnsiColorWriter(os.Stdout))
	//log.AddHook(&journalhook.JournalHook{})
	//journalhook.Enable()

	if !flagRunBot && !flagRunWeb && flagRunFeeds == "" && !flagRunEverything && flagAction == "" && !flagDryRun {
		log.Error("Didnt specify what to run, see -h for more info")
		return
	}

	if flagRunWeb || flagRunEverything {
		common.RedisPoolSize = 25
	}
	if flagRunBot || flagRunEverything {
		common.RedisPoolSize = 100
	}

	log.Info("YAGPDB is initializing...")

	err := common.Init()
	if err != nil {
		log.WithError(err).Fatal("Failed intializing")
	}

	configstore.InitDatabases()

	//BotSession.LogLevel = discordgo.LogInformational

	// Setup plugins
	discordlogger.Register()
	docs.RegisterPlugin()
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
	automod.RegisterPlugin()
	logs.InitPlugin()
	autorole.RegisterPlugin()
	reminders.RegisterPlugin()
	soundboard.RegisterPlugin()
	youtube.RegisterPlugin()
	rolecommands.RegisterPlugin()

	if flagDryRun {
		log.Println("This is a dry run, exiting")
		return
	}

	// Setup plugins for bot, but run later if enabled
	bot.Setup()
	mqueue.InitStores()

	// RUN FORREST RUN
	if flagAction != "" {
		runAction(flagAction)
		return
	}

	if flagRunWeb || flagRunEverything {
		go web.Run()
	}

	if flagRunBot || flagRunEverything {
		bot.Run()
		go botrest.StartServer()
		go mqueue.StartPolling()
	}

	if flagRunFeeds != "" || flagRunEverything {
		go feeds.Run(strings.Split(flagRunFeeds, ","))
	}

	go pubsub.PollEvents()

	listenSignal()
}

func runAction(str string) {
	log.Info("Running action", str)
	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed to get redis connection")
		return
	}
	defer common.RedisPool.Put(client)

	switch str {
	case "connected":
		err = common.RefreshConnectedGuilds(common.BotSession, client)
	case "rsconnected":
		err = client.Cmd("DEL", "connected_guilds").Err
	case "migrate":
		err = migrate(client)
	default:
		log.Error("Unknown action")
		return
	}

	if err != nil {
		log.WithError(err).Error("Error running action")
	} else {
		log.Info("Sucessfully ran action", str)
	}
}

// Gracefull shutdown
func listenSignal() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Info("SHUTTING DOWN...")

	shouldWait := false
	var wg sync.WaitGroup

	if flagRunBot || flagRunEverything {

		wg.Add(2)

		go bot.Stop(&wg)
		go scheduledevents.Stop(&wg)
		mqueue.Stop(&wg)

		shouldWait = true
	}

	if flagRunFeeds != "" || flagRunEverything {
		feeds.Stop(&wg)
		shouldWait = true
	}

	if flagRunWeb {
		web.Stop()
		// Slep for a extra second
		time.Sleep(time.Second)
	}

	if shouldWait {
		log.Info("Waiting for things to shut down...")
		wg.Wait()
	}

	log.Info("Sleeping for a second to allow work to finish")
	time.Sleep(time.Second)

	if !common.Testing {
		log.Info("Sleeping a little longer")
		time.Sleep(time.Second * 5)
	}

	log.Info("Bye..")
	os.Exit(0)
}

type SQLMigrater interface {
	MigrateStorage(client *redis.Client, guildIDInt int64) error
	Name() string
}

func migrate(client *redis.Client) error {
	plugins := make([]SQLMigrater, 0)

	for _, v := range common.Plugins {
		cast, ok := v.(SQLMigrater)
		if ok {
			plugins = append(plugins, cast)
			log.Info("Migrating ", cast.Name())
		}
	}

	guilds, err := client.Cmd("SMEMBERS", "connected_guilds").List()
	if err != nil {
		return err
	}

	started := time.Now()
	for _, g := range guilds {

		parsed, err := strconv.ParseInt(g, 10, 64)
		if err != nil {
			return err
		}

		for _, p := range plugins {
			err = p.MigrateStorage(client, parsed)
			if err != nil {
				log.WithError(err).Error("Error migrating ", p.Name())
			}
		}
	}
	elapsed := time.Since(started)
	log.Info("Migrated ", len(guilds), " guilds in ", elapsed.String())

	return nil
}
