package orchestrator

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
)

type monitor struct {
	orchestrator *Orchestrator

	started  time.Time
	stopChan chan bool

	lastTimeLaunchedNode       time.Time
	lastTimeStartedShardBucket time.Time
	shardsLastSeenTimes        []time.Time
}

func (mon *monitor) run() {
	mon.started = time.Now()
	ticker := time.NewTicker(time.Second)

	// allows time for shards to connect before we start launching stuff
	time.Sleep(time.Second * 10)

	for {
		select {
		case <-ticker.C:
			mon.tick()
		case <-mon.stopChan:
			return
		}
	}
}

func (mon *monitor) stop() {
	close(mon.stopChan)
}

func (mon *monitor) ensureTotalShards() int {
	mon.orchestrator.mu.Lock()
	defer mon.orchestrator.mu.Unlock()

	// totalshards has been set already
	totalShards := mon.orchestrator.totalShards
	if totalShards != 0 {
		return totalShards
	}

	// has not been set, try to set it
	totalShards, err := mon.orchestrator.getTotalShardCount()
	if err != nil {
		mon.orchestrator.Log(dshardorchestrator.LogError, err, "monitor: failed fetching total shard count, retrying in a second")
		return 0
	}

	if totalShards == 0 {
		mon.orchestrator.Log(dshardorchestrator.LogError, err, "monitor: ShardCountProvider returned 0 without error, retrying in a second")
		return 0
	}

	// successfully set it
	mon.orchestrator.totalShards = totalShards
	mon.orchestrator.Log(dshardorchestrator.LogInfo, nil, fmt.Sprintf("monitor: set total shard count to %d", totalShards))
	return totalShards
}

func (mon *monitor) tick() {
	if !mon.orchestrator.EnsureAllShardsRunning {
		// currently this is the only purpose of the monitor, it may be extended to perform more as it could be a reliable way of handling a bunch of things
		return
	}

	totalShards := mon.ensureTotalShards()
	if totalShards == 0 {
		return
	}

	if time.Since(mon.lastTimeStartedShardBucket) < time.Second*5 {
		return
	}

	if mon.shardsLastSeenTimes == nil {
		mon.shardsLastSeenTimes = make([]time.Time, totalShards)
		for i := range mon.shardsLastSeenTimes {
			mon.shardsLastSeenTimes[i] = time.Now()
		}
	}

	runningShards := make([]bool, totalShards)

	// find the disconnect times for all shards that have disconnected
	fullNodeStatuses := mon.orchestrator.GetFullNodesStatus()
	for _, ns := range fullNodeStatuses {
		for _, s := range ns.Shards {
			if !ns.Connected {
				continue
			}

			runningShards[s] = true
			mon.shardsLastSeenTimes[s] = time.Now()
		}
	}

	// find out which shards to start, the key is the bucket
	shardsToStart := make(map[int][]int)
	// shardsToStart := make([]int, 0)
	nToStart := 0

OUTER:
	for i, lastTimeConnected := range mon.shardsLastSeenTimes {
		if runningShards[i] || time.Since(lastTimeConnected) < mon.orchestrator.MaxNodeDowntimeBeforeRestart {
			continue
		}

		// check if this shard is in a shard migration, in which case ignore it
		for _, ns := range fullNodeStatuses {
			if ns.MigratingShard == i && (ns.MigratingFrom != "" || ns.MigratingTo != "") {
				continue OUTER
			}
		}

		// if were running multi-host mode, check to make sure we don't start a shard were not responsible for
		if !mon.orchestrator.isResponsibleForShard(i) {
			continue
		}

		bucket := mon.nodeSlotForShard(i)

		shardsToStart[bucket] = append(shardsToStart[bucket], i)
		nToStart++
	}

	if len(shardsToStart) < 1 {
		return
	}

	mon.orchestrator.Log(dshardorchestrator.LogInfo, nil, fmt.Sprintf("monitor: need to start %d shards...", nToStart))

	// start one
	// reason we don't start them all at once is that there's no real point in that, you can only start a shard every 5 seconds anyways
	// attempt to start it on a existing node before starting a new node
	for _, v := range fullNodeStatuses {
		if !v.Connected || !v.SessionEstablished || v.Blacklisted {
			continue
		}

		if len(v.Shards) >= mon.orchestrator.MaxShardsPerNode {
			continue
		}

		canStartBucket := -1

	FINDAVAILBUCKET:
		for bucket := range shardsToStart {
			for _, vs := range v.Shards {
				vb := mon.nodeSlotForShard(vs)
				if bucket != vb {
					// mistmatched buckets, can't start this shard here
					continue FINDAVAILBUCKET
				}
			}

			// no mismatched shard buckets
			canStartBucket = bucket
		}

		mon.orchestrator.Log(dshardorchestrator.LogInfo, nil, fmt.Sprintf("monitor: canStartBucket: %d", canStartBucket))

		if canStartBucket == -1 {
			continue
		}

		starting := shardsToStart[canStartBucket]

		if mon.orchestrator.ShardBucketSize != 0 && len(starting) > mon.orchestrator.ShardBucketSize {
			starting = starting[:mon.orchestrator.ShardBucketSize]
		}

		mon.orchestrator.Log(dshardorchestrator.LogInfo, nil, fmt.Sprintf("monitor: starting: %v", starting))

		err := mon.orchestrator.StartShards(v.ID, starting...)
		if err != nil {
			mon.orchestrator.Log(dshardorchestrator.LogError, err, "monitor: failed starting shards")
		}
		mon.lastTimeStartedShardBucket = time.Now()
		return
	}

	// if we got here that means that there's no more nodes available, so start one
	if time.Since(mon.lastTimeLaunchedNode) < time.Second*30 {
		// allow 5 seconds wait time in between each node launch
		mon.orchestrator.Log(dshardorchestrator.LogDebug, nil, "monitor: can't start new node, on cooldown")
		return
	}

	if mon.orchestrator.NodeLauncher == nil {
		mon.orchestrator.Log(dshardorchestrator.LogError, nil, "monitor: can't start new node, no node launcher set...")
		return
	}

	_, err := mon.orchestrator.NodeLauncher.LaunchNewNode()
	if err != nil {
		mon.orchestrator.Log(dshardorchestrator.LogError, err, "monitor: failed starting a new node")
		return
	}

	mon.lastTimeLaunchedNode = time.Now()
}

func (mon *monitor) bucketForShard(shard int) int {
	bs := mon.orchestrator.ShardBucketSize
	if bs > 1 {
		return shard / bs
	}

	// not using buckets
	return 0
}

// same as bucketForShard but also applies the bucketsPerNode to for assigning buckets to nodes
func (mon *monitor) nodeSlotForShard(shard int) int {
	bucket := mon.bucketForShard(shard)
	if mon.orchestrator.BucketsPerNode != 0 {
		return bucket / mon.orchestrator.BucketsPerNode
	}

	// buckets per node is not set, ignore
	return bucket
}
