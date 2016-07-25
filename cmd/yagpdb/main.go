package main

import (
	"flag"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/web"
	"log"
)

var (
	flagRunBot    bool
	flagRunWeb    bool
	flagRunReddit bool

	flagAction string

	flagRunEverything bool
	flagAddr          string
	flagConfig        string

	BotSession *discordgo.Session
	RedisPool  *pool.Pool
)

func init() {
	flag.BoolVar(&flagRunBot, "bot", false, "Set to run discord bot")
	flag.BoolVar(&flagRunWeb, "web", false, "Set to run webserver")
	flag.BoolVar(&flagRunReddit, "reddit", false, "Set to run reddit bot")

	flag.BoolVar(&flagRunEverything, "all", false, "Set to everything (discord bot, webserver and reddit bot)")

	flag.StringVar(&flagAddr, "addr", ":5000", "Address for webserver to listen on")
	flag.StringVar(&flagConfig, "conf", "config.json", "Path to config file")
	flag.StringVar(&flagAction, "a", "", "Run a action and exit, available actions: connected")
	flag.Parse()
}

func main() {

	if !flagRunBot && !flagRunWeb && !flagRunReddit && !flagRunEverything && flagAction == "" {
		log.Println("Didnt specify what to run, see -h for more info")
		return
	}

	log.Println("YAGPDB is initializing...")
	config, err := common.LoadConfig(flagConfig)
	if err != nil {
		log.Println("Failed loading config", err)
		return
	}

	BotSession, err = discordgo.New(config.BotToken)
	if err != nil {
		log.Println("Error intializing bot session:", err)
		return
	}
	BotSession.MaxRestRetries = 3
	//BotSession.LogLevel = discordgo.LogInformational

	RedisPool, err = pool.NewPool("tcp", config.Redis, 10)
	if err != nil {
		log.Println("Failed initializing redis pool")
		return
	}

	if flagAction != "" {
		runAction(flagAction)
		return
	}

	common.RedisPool = RedisPool
	common.Conf = config
	common.BotSession = BotSession

	// Setup plugins
	serverstats.RegisterPlugin()
	notifications.RegisterPlugin()
	commands.RegisterPlugin()
	customcommands.RegisterPlugin()
	reddit.RegisterPlugin()

	// RUN FORREST RUN
	if flagRunWeb || flagRunEverything {
		web.ListenAddress = flagAddr
		go web.Run()
	}

	if flagRunBot || flagRunEverything {
		go bot.Run()
	}

	if flagRunReddit || flagRunEverything {
		go reddit.RunReddit()
	}

	select {}
}

func runAction(str string) {
	log.Println("Running action", str)
	client, err := RedisPool.Get()
	if err != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer RedisPool.CarefullyPut(client, &err)

	switch str {
	case "connected":
		err = common.RefreshConnectedGuilds(BotSession, client)
	default:
		log.Println("Unknown action")
		return
	}

	if err != nil {
		log.Println("Error:", err)
	} else {
		log.Println("Sucessfully ran action", str)
	}
}
