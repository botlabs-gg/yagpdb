package main

import (
	"flag"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/web"
	"log"
)

var (
	flagMode string
)

func init() {
	flag.StringVar(&flagMode, "mode", "", "The mode to run yagpdb (web, bot, both)")

	flag.Parse()
}

func main() {
	config, err := common.LoadConfig("config.json")
	if err != nil {
		log.Println("Failed loading config", err)
		return
	}
	web.Config = config

	// Setup plugins
	serverstats.RegisterPlugin()

	// RUN FOREST RUN
	web.Run()
}
