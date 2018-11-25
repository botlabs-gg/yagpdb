package main

import (
	"github.com/jonas747/dshardorchestrator"
	"github.com/jonas747/dshardorchestrator/orchestrator"
	"github.com/jonas747/dshardorchestrator/orchestrator/rest"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"log"
	"time"

	_ "github.com/jonas747/yagpdb/bot" // register the custom orchestrator events
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors: true,
	})

	err := common.Init()
	if err != nil {
		panic("failed initializing: " + err.Error())
	}

	orch := orchestrator.NewStandardOrchestrator(common.BotSession)
	orch.NodeLauncher = &orchestrator.StdNodeLauncher{
		CmdName: "./capturepanics",
		Args:    []string{"./yagpdb", "-bot", "-syslog"},
	}
	orch.Logger = &dshardorchestrator.StdLogger{
		Level: dshardorchestrator.LogDebug,
	}
	orch.ShardCountProvider = &RedisShardCountProvider{Key: "dshardorchestrator_totalshards"}

	orch.MaxShardsPerNode = 10
	orch.MaxNodeDowntimeBeforeRestart = time.Second * 10
	orch.EnsureAllShardsRunning = true

	err = orch.Start("127.0.0.1:7447")
	if err != nil {
		log.Fatal("failed starting orchestrator: ", err)
	}

	go UpdateRedisNodes(orch)

	api := rest.NewRESTAPI(orch, "127.0.0.1:7448")
	err = api.Run()
	if err != nil {
		log.Fatal("failed starting rest api: ", err)
	}

	select {}
}

// RedisShardCountProvider  is a that queries redis for total shard count
type RedisShardCountProvider struct {
	Key string
}

func (sc *RedisShardCountProvider) GetTotalShardCount() (int, error) {
	shards := 0
	err := common.RedisPool.Do(radix.Cmd(&shards, "GET", sc.Key))
	return shards, err
}

const RedisNodesKey = "dshardorchestrator_nodes"

func UpdateRedisNodes(orch *orchestrator.Orchestrator) {

	t := time.NewTicker(time.Second * 10)
	for {
		<-t.C

		fullStatus := orch.GetFullNodesStatus()
		common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisNodesKey+"_new"))

		addedAny := false
		for _, v := range fullStatus {
			if !v.Connected {
				continue
			}

			addedAny = true
			common.RedisPool.Do(radix.Cmd(nil, "SADD", RedisNodesKey+"_new", v.ID))
		}

		if addedAny {
			common.RedisPool.Do(radix.Cmd(nil, "RENAME", RedisNodesKey+"_new", RedisNodesKey))
		} else {
			common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisNodesKey))
		}
	}
}
