package main

import (
	"log"
	"os"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator/rest"
)

func main() {
	discordSession, err := discordgo.New(os.Getenv("DG_TOKEN"))
	if err != nil {
		log.Fatal("failed initilazing discord session: ", err)
	}

	orch := orchestrator.NewStandardOrchestrator(discordSession)
	orch.NodeLauncher = orchestrator.NewNodeLauncher("./dso-sample-bot", nil, nil)

	orch.Logger = &dshardorchestrator.StdLogger{
		Level: dshardorchestrator.LogDebug,
	}
	orch.ShardCountProvider = &mockShardCountProvider{
		NumShards: 10,
	}

	orch.MaxShardsPerNode = 5
	orch.MaxNodeDowntimeBeforeRestart = time.Second * 10
	orch.EnsureAllShardsRunning = true

	err = orch.Start("127.0.0.1:7447")
	if err != nil {
		log.Fatal("failed starting orchestrator: ", err)
	}

	api := rest.NewRESTAPI(orch, "127.0.0.1:7448")
	err = api.Run()
	if err != nil {
		log.Fatal("failed starting rest api: ", err)
	}

	select {}
}

type mockShardCountProvider struct {
	NumShards int
}

func (m *mockShardCountProvider) GetTotalShardCount() (int, error) {
	return m.NumShards, nil
}
