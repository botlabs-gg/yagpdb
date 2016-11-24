package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/automod"
	"github.com/jonas747/yagpdb/autorole"
	"github.com/jonas747/yagpdb/aylien"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/logs"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/reminders"
	"github.com/jonas747/yagpdb/reputation"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/streaming"
	"github.com/jonas747/yagpdb/web"
	"sync"
	//"github.com/wercker/journalhook"
)

var (
	flagRunBot    bool
	flagRunWeb    bool
	flagRunReddit bool
	flagRunStats  bool

	flagAction string

	flagRunEverything bool
	flagLogTimestamp  bool
	flagAddr          string
	flagConfig        string
)

func init() {
	flag.BoolVar(&flagRunBot, "bot", false, "Set to run discord bot and bot related stuff")
	flag.BoolVar(&flagRunWeb, "web", false, "Set to run webserver")
	flag.BoolVar(&flagRunReddit, "reddit", false, "Set to run reddit bot")
	flag.BoolVar(&flagRunEverything, "all", false, "Set to everything (discord bot, webserver and reddit bot)")

	flag.BoolVar(&flagLogTimestamp, "ts", false, "Set to include timestamps in log")
	flag.StringVar(&flagAddr, "addr", ":5001", "Address for webserver to listen on")
	flag.StringVar(&flagConfig, "conf", "config.json", "Path to config file")
	flag.StringVar(&flagAction, "a", "", "Run a action and exit, available actions: connected")
	flag.Parse()
}

func main() {

	log.AddHook(common.ContextHook{})
	//log.AddHook(&journalhook.JournalHook{})
	//journalhook.Enable()

	if flagLogTimestamp {
		web.LogRequestTimestamps = true
	}

	if !flagRunBot && !flagRunWeb && !flagRunReddit && !flagRunEverything && flagAction == "" {
		log.Error("Didnt specify what to run, see -h for more info")
		return
	}

	log.Info("YAGPDB is initializing...")

	err := common.Init(flagConfig)
	if err != nil {
		log.WithError(err).Fatal("Failed intializing")
	}

	//BotSession.LogLevel = discordgo.LogInformational
	if flagAction != "" {
		runAction(flagAction)
		return
	}

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

	// Setup plugins for bot, but run later if enabled
	bot.Setup()

	// RUN FORREST RUN
	if flagRunWeb || flagRunEverything {
		go web.Run()
	}

	if flagRunBot || flagRunEverything {
		go bot.Run()
		go serverstats.UpdateStatsLoop()
		go common.RunScheduledEvents(make(chan *sync.WaitGroup))
	}

	if flagRunReddit || flagRunEverything {
		go reddit.RunReddit()
	}

	select {}
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
