package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	dshardmanager "github.com/botlabs-gg/yagpdb/v2/lib/jdshardmanager"
)

var (
	FlagToken      string
	FlagLogChannel int64
)

func main() {

	flag.StringVar(&FlagToken, "t", "", "Discord token")
	flag.Int64Var(&FlagLogChannel, "c", 0, "Log channel, optional")
	flag.Parse()

	log.Println("Starting v" + dshardmanager.VersionString)
	if FlagToken == "" {
		FlagToken = os.Getenv("DG_TOKEN")
		if FlagToken == "" {
			log.Fatal("No token specified")
		}
	}

	if !strings.HasPrefix(FlagToken, "Bot ") {
		log.Fatal("dshardmanager only works on bot accounts, did you maybe forgot to add `Bot ` before the token?")
	}

	manager := dshardmanager.New(FlagToken)
	manager.Name = "ExampleBot"
	manager.LogChannel = FlagLogChannel
	manager.StatusMessageChannel = FlagLogChannel

	recommended, err := manager.GetRecommendedCount()
	if err != nil {
		log.Fatal("Failed getting recommended shard count")
	}
	if recommended < 2 {
		manager.SetNumShards(5)
	}

	log.Println("Starting the shard manager")
	err = manager.Start()
	if err != nil {
		log.Fatal("Faled to start: ", err)
	}

	log.Println("Started!")

	log.Fatal(http.ListenAndServe(":7441", nil))
	select {}
}
