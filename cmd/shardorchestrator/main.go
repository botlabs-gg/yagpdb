package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator/rest"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"

	_ "github.com/botlabs-gg/yagpdb/v2/bot" // register the custom orchestrator events
)

var ()

func main() {
	common.RedisPoolSize = 2
	err := common.CoreInit(true)
	if err != nil {
		panic("failed initializing: " + err.Error())
	}

	activeShards := ReadActiveShards()
	totalShards := common.ConfTotalShards.GetInt()
	if totalShards < 1 {
		panic("YAGPDB_SHARDING_TOTAL_SHARDS needs to be set to a resonable number of total shards")
	}

	if len(activeShards) < 1 {
		panic("YAGPDB_SHARDING_ACTIVE_SHARDS is not set, needs to be set to the shards that should be active on this host, ex: '1-49,60-99'")
	}

	logrus.Info("Running shards (", len(activeShards), "): ", activeShards)
	logrus.Info("Large bot sharding: ", common.ConfLargeBotShardingEnabled.GetBool())

	orch := orchestrator.NewStandardOrchestrator(common.BotSession)
	orch.FixedTotalShardCount = totalShards
	orch.ResponsibleForShards = activeShards
	orch.NodeLauncher = &orchestrator.StdNodeLauncher{
		LaunchCmdName: "./capturepanics",
		LaunchArgs:    []string{"./yagpdb", "-bot", "-syslog"},

		VersionCmdName: "./yagpdb",
		VersionArgs:    []string{"-version"},
	}
	orch.Logger = &dshardorchestrator.StdLogger{
		Level: dshardorchestrator.LogInfo,
	}

	if common.ConfLargeBotShardingEnabled.GetBool() {
		orch.ShardBucketSize = common.ConfShardBucketSize.GetInt()
		orch.BucketsPerNode = common.ConfBucketsPerNode.GetInt()
	} else {
		orch.ShardBucketSize = 1
		orch.BucketsPerNode = common.ConfBucketsPerNode.GetInt()
	}

	updateScript := "updateversion.sh"
	orch.VersionUpdater = &Updater{
		ScriptLocation: updateScript,
		orchestrator:   orch,
	}

	orch.MaxShardsPerNode = orch.ShardBucketSize * orch.BucketsPerNode
	orch.MaxNodeDowntimeBeforeRestart = time.Second * 10
	orch.EnsureAllShardsRunning = true

	logrus.Infof("%#v+n", *orch)

	err = orch.Start("127.0.0.1:7447")
	if err != nil {
		log.Fatal("failed starting orchestrator: ", err)
	}

	go UpdateRedisNodes(orch)

	restAPIAddr := os.Getenv("YAGPDB_BOTREST_LISTEN_ADDRESS")
	if restAPIAddr == "" {
		restAPIAddr = "127.0.0.1"
	}

	api := rest.NewRESTAPI(orch, restAPIAddr+":7448")
	common.ServiceTracker.SetAPIAddress(restAPIAddr + ":7448")
	common.ServiceTracker.RegisterService(common.ServiceTypeOrchestator, "Shard orchestrator", "", nil)

	err = api.Run()
	if err != nil {
		log.Fatal("failed starting rest api: ", err)
	}

	select {}
}

const RedisNodesKey = "dshardorchestrator_nodes_z"

func UpdateRedisNodes(orch *orchestrator.Orchestrator) {

	t := time.NewTicker(time.Second * 10)
	for {
		<-t.C

		fullStatus := orch.GetFullNodesStatus()

		for _, v := range fullStatus {
			if !v.Connected {
				continue
			}

			err := common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", RedisNodesKey, time.Now().Unix(), v.ID))
			if err != nil {
				logrus.WithError(err).Error("[orchestrator] failed setting active nodes in redis")
			}
		}
	}
}

func ReadActiveShards() []int {
	str := common.ConfActiveShards.GetString()
	split := strings.Split(str, ",")

	shards := make([]int, 0)
	for _, v := range split {
		if strings.Contains(v, "-") {
			minMaxSplit := strings.Split(v, "-")
			if len(minMaxSplit) < 2 {
				panic("Invalid min max format in active shards: " + v)
			}

			min, err := strconv.Atoi(strings.TrimSpace(minMaxSplit[0]))
			if err != nil {
				panic("Invalid number min, in active shards: " + v + ", " + err.Error())
			}

			max, err := strconv.Atoi(strings.TrimSpace(minMaxSplit[1]))
			if err != nil {
				panic("Invalid number max, in active shards: " + v + ", " + err.Error())
			}

			for i := min; i <= max; i++ {
				shards = append(shards, i)
			}
		} else {
			parsed, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				panic("Invalid shard number in active shards: " + v + ", " + err.Error())
			}

			shards = append(shards, parsed)
		}
	}

	return shards
}

type Updater struct {
	ScriptLocation string
	orchestrator   *orchestrator.Orchestrator
}

func (u *Updater) PullNewVersion() (string, error) {
	err := os.Mkdir("updating", 0770)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	logrus.Println("Updatig version")
	cmd := exec.Command("/bin/bash", u.ScriptLocation)
	cmd.Dir = "updating/"

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	logrus.Println("Update output: ", output)
	return u.orchestrator.NodeLauncher.LaunchVersion()
}
