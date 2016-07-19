package main

import (
	"flag"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/web"
	"log"
)

var (
	flagMode   string
	flagAction string

	BotSession *discordgo.Session
	RedisPool  *pool.Pool
)

func init() {
	flag.StringVar(&flagMode, "m", "both", "The mode to run yagpdb (web, bot, both)")
	flag.StringVar(&flagAction, "a", "", "Run a action and exit, available actions: connected")
	flag.Parse()
}

func main() {
	if flagMode != "bot" && flagMode != "web" && flagMode != "both" {
		log.Println("mode (-m) has to be one of bot, web and both")
		return
	}

	log.Println("YAGPDB is initializing...")
	config, err := common.LoadConfig("config.json")
	if err != nil {
		log.Println("Failed loading config", err)
		return
	}
	web.Config = config
	bot.Config = config

	BotSession, err = discordgo.New(config.BotToken)
	if err != nil {
		log.Println("Error intializing bot session:", err)
		return
	}
	bot.Session = BotSession

	RedisPool, err = pool.NewPool("tcp", config.Redis, 10)
	if err != nil {
		log.Println("Failed initializing redis pool")
		return
	}

	web.RedisPool = RedisPool
	bot.RedisPool = RedisPool

	if flagAction != "" {
		runAction(flagAction)
		return
	}

	// Setup plugins
	serverstats.RegisterPlugin()

	// RUN FOREST RUN
	if flagMode == "web" || flagMode == "both" {
		go web.Run()
	}

	if flagMode == "bot" || flagMode == "both" {
		go bot.Run()
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
