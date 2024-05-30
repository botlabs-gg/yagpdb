package main

import (
	"flag"
	"log"
	"os"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
)

var Node *node.Conn

var FlagNodeID string

func init() {
	flag.StringVar(&FlagNodeID, "nodeid", "", "the node id")
	flag.Parse()
}

func main() {
	if FlagNodeID == "" {
		log.Fatal("no -nodeid provided")
	}

	bot := &Bot{
		token: os.Getenv("DG_TOKEN"),
	}

	n, err := node.ConnectToOrchestrator(bot, "127.0.0.1:7447", "example.1", FlagNodeID, &dshardorchestrator.StdLogger{
		Level: dshardorchestrator.LogDebug,
	})
	if err != nil {
		log.Fatal("failed connecting to orchestrator")
	}

	Node = n

	select {}
}
