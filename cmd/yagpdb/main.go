package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/automod"
	"github.com/jonas747/yagpdb/autorole"
	"github.com/jonas747/yagpdb/aylien"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/jonas747/yagpdb/logs"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/reminders"
	"github.com/jonas747/yagpdb/reputation"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/soundboard"
	"github.com/jonas747/yagpdb/streaming"
	"github.com/jonas747/yagpdb/web"
	"github.com/shiena/ansicolor"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
	//"github.com/wercker/journalhook"
)

var (
	flagRunBot        bool
	flagRunWeb        bool
	flagRunFeeds      bool
	flagRunEverything bool
	flagDryRun        bool

	flagAction string

	flagLogTimestamp bool
)

func init() {
	flag.BoolVar(&flagRunBot, "bot", false, "Set to run discord bot and bot related stuff")
	flag.BoolVar(&flagRunWeb, "web", false, "Set to run webserver")
	flag.BoolVar(&flagRunFeeds, "feeds", false, "Set to run feeds")
	flag.BoolVar(&flagRunEverything, "all", false, "Set to everything (discord bot, webserver and reddit bot)")
	flag.BoolVar(&flagDryRun, "dry", false, "Do a dryrun, initialize all plugins but don't actually start anything")

	flag.BoolVar(&flagLogTimestamp, "ts", false, "Set to include timestamps in log")
	flag.StringVar(&flagAction, "a", "", "Run a action and exit, available actions: connected")
}

func main() {
	flag.Parse()

	log.AddHook(common.ContextHook{})
	log.SetOutput(ansicolor.NewAnsiColorWriter(os.Stdout))
	//log.AddHook(&journalhook.JournalHook{})
	//journalhook.Enable()

	if flagLogTimestamp {
		web.LogRequestTimestamps = true
	}

	if !flagRunBot && !flagRunWeb && !flagRunFeeds && !flagRunEverything && flagAction == "" && !flagDryRun {
		log.Error("Didnt specify what to run, see -h for more info")
		return
	}

	log.Info("YAGPDB is initializing...")

	err := common.Init()
	if err != nil {
		log.WithError(err).Fatal("Failed intializing")
	}

	configstore.InitDatabases()

	//BotSession.LogLevel = discordgo.LogInformational

	// Setup plugins
	commands.RegisterPlugin()
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

	if flagDryRun {
		log.Println("This is a dry run, exiting")
		return
	}

	// Setup plugins for bot, but run later if enabled
	bot.Setup()

	// RUN FORREST RUN
	if flagAction != "" {
		runAction(flagAction)
		return
	}

	if flagRunWeb || flagRunEverything {
		go web.Run()
	}

	if flagRunBot || flagRunEverything {
		go bot.Run()
		go common.RunScheduledEvents()
	}

	if flagRunFeeds || flagRunEverything {
		go feeds.Run()
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
	defer common.RedisPool.CarefullyPut(client, &err)

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
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	<-c
	log.Info("SHUTTING DOWN...")

	shouldWait := false
	var wg sync.WaitGroup

	if flagRunBot || flagRunEverything {

		wg.Add(2)

		go bot.Stop(&wg)
		go common.StopSheduledEvents(&wg)

		shouldWait = true
	}

	if flagRunFeeds || flagRunEverything {
		feeds.Stop(&wg)
		shouldWait = true
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

type SQLMigrater interface {
	MigrateStorage(client *redis.Client, guildID string, guildIDInt int64) error
	Name() string
}

func migrate(client *redis.Client) error {
	plugins := make([]SQLMigrater, 0)

	for _, v := range bot.Plugins {
		cast, ok := v.(SQLMigrater)
		if ok {
			plugins = append(plugins, cast)
			log.Info("Migrating ", cast.Name())
		}
	}

OUTER:
	for _, v := range web.Plugins {
		for _, p := range plugins {
			if interface{}(v) == p {
				log.Info("Found duplicate ", v.Name())
				continue OUTER
			}
		}

		if cast, ok := v.(SQLMigrater); ok {
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
			err = p.MigrateStorage(client, g, parsed)
			if err != nil {
				log.WithError(err).Error("Error migrating ", p.Name())
			}
		}
	}
	elapsed := time.Since(started)
	log.Info("Migrated ", len(guilds), " guilds in ", elapsed.String())

	return nil
}
