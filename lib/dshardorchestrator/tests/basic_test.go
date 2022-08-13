package tests

import (
	"sync"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator"
)

var testServerAddr = "127.0.0.1:7447"
var testLoggerNode = &dshardorchestrator.StdLogger{Level: dshardorchestrator.LogDebug, Prefix: "node: "}
var testLoggerOrchestrator = &dshardorchestrator.StdLogger{Level: dshardorchestrator.LogInfo, Prefix: "orchestrator: "}

type mockShardCountProvider struct {
	NumShards int
}

func (m *mockShardCountProvider) GetTotalShardCount() (int, error) {
	return m.NumShards, nil
}

func CreateMockOrchestrator(numShards int) *orchestrator.Orchestrator {
	return &orchestrator.Orchestrator{
		ShardCountProvider: &mockShardCountProvider{numShards},
		// NodeIDProvider:     orchestrator.NewNodeIDProvider(),
		Logger:                        testLoggerOrchestrator,
		SkipSafeStartupDelayMaxShards: true,
	}
}

func TestEstablishSession(t *testing.T) {
	orchestrator := CreateMockOrchestrator(10)
	err := orchestrator.Start(testServerAddr)
	if err != nil {
		t.Fatal("failed starting orchestrator: ", err)
		return
	}
	defer orchestrator.Stop()

	waitChan := make(chan node.SessionInfo)
	bot := &MockBot{
		SessionEstablishedFunc: func(info node.SessionInfo) {
			waitChan <- info
		},
	}

	n, err := node.ConnectToOrchestrator(bot, testServerAddr, "testing", generateID(), testLoggerNode)
	if err != nil {
		t.Fatal("failed connecting to orchestrator: ", err)
	}
	defer n.Close()

	select {
	case info := <-waitChan:
		if info.TotalShards != 10 {
			t.Error("mismatched total shards: ", info.TotalShards)
		}
	case <-time.After(time.Second * 15):
		t.Fatal("timed out waiting for session to be established")
	}
}

func startConnectNode(t *testing.T, bot node.Interface, sessionWaitChan chan node.SessionInfo) (*node.Conn, bool) {
	n, err := node.ConnectToOrchestrator(bot, testServerAddr, "testing", generateID(), testLoggerNode)
	if err != nil {
		n.Close()
		t.Fatal("failed connecting to orchestrator: ", err)
		return nil, false
	}

	select {
	case info := <-sessionWaitChan:
		if info.TotalShards != 10 {
			t.Fatal("mismatched total shards: ", info.TotalShards)
			n.Close()
		}
	case <-time.After(time.Second * 15):
		n.Close()
		t.Fatal("timed out waiting for session to be established")
	}

	return n, true
}

func TestMigrateShard(t *testing.T) {
	orchestrator := CreateMockOrchestrator(10)
	err := orchestrator.Start(testServerAddr)
	if err != nil {
		t.Fatal("failed starting orchestrator: ", err)
		return
	}
	defer orchestrator.Stop()

	dataToMigrate := []string{
		"hello world",
		"is this working?",
		"i hope so",
	}

	dshardorchestrator.RegisterUserEvent("test", 101, "")

	// set up the sessions
	var n1 *node.Conn

	sessionWaitChan := make(chan node.SessionInfo)
	shardStartedChan := make(chan int)
	shardsAddedChan := make(chan []int, 10)

	bot1 := &MockBot{
		SessionEstablishedFunc: func(info node.SessionInfo) {
			sessionWaitChan <- info
		},
		ResumeShardFunc: func(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
			shardStartedChan <- shard
		},
		AddNewShardFunc: func(shards ...int) {
			shardsAddedChan <- shards
		},
		StartShardTransferFromFunc: func(shard int) int {
			for _, v := range dataToMigrate {
				go n1.SendLogErr(101, v, false)
			}

			return len(dataToMigrate)
		},
	}

	dataReceived := make([]bool, len(dataToMigrate))
	var dataReceivedMU sync.Mutex

	bot2 := &MockBot{
		SessionEstablishedFunc: func(info node.SessionInfo) {
			sessionWaitChan <- info
		},
		ResumeShardFunc: func(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
			dataReceivedMU.Lock()
			for i, v := range dataReceived {
				if !v {
					t.Errorf("data did not get migrated migrated #%d: %v", i, dataToMigrate[i])
				}
			}
			dataReceivedMU.Unlock()
			shardStartedChan <- shard
		},
		AddNewShardFunc: func(shards ...int) {
			shardsAddedChan <- shards
		},
		HandleUserEventFunc: func(evt dshardorchestrator.EventType, data interface{}) {
			dataCast := *data.(*string)
			if evt != 101 {
				t.Fatal("event was not 101")
			}

			dataReceivedMU.Lock()
			for i, v := range dataToMigrate {
				if v == dataCast {
					dataReceived[i] = true
				}
			}
			dataReceivedMU.Unlock()
		},
	}

	// connect node 1
	n1, ok := startConnectNode(t, bot1, sessionWaitChan)
	defer n1.Close()

	if !ok {
		t.Fail()
		return
	}

	// connect node 2
	n2, ok := startConnectNode(t, bot2, sessionWaitChan)
	defer n2.Close()

	if !ok {
		t.Fail()
		return
	}

	on1 := orchestrator.FindNodeByID(n1.GetIDLock())

	// start 5 shards on node 1
	addWaitForShards(t, []int{0, 1, 2, 3, 4}, shardsAddedChan, on1)

	// make sure that the orcehstrator has gotten feedback that the shards have started
	time.Sleep(time.Millisecond * 250)

	// perform the migration
	err = orchestrator.StartShardMigration(n2.GetIDLock(), 3)
	if err != nil {
		t.Fatal("failed performing migration: ", err.Error())
	}

	select {
	case s := <-shardStartedChan:
		if s != 3 {
			t.Fatal("mismatched shard id")
			return
		}
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for shard to start after migration")
	}

	// sleep another 250 milliseconds again to allow time for the shard orchestrator to acknowledge that its started
	time.Sleep(time.Millisecond * 250)

	// confirm the setup
	statuses := orchestrator.GetFullNodesStatus()
	for _, v := range statuses {
		if len(v.Shards) < 1 {
			t.Fatal("node holds no shards: ", v.ID)
		}

		if v.ID == n2.GetIDLock() {
			if len(v.Shards) != 1 {
				t.Fatal("node 2 holds incorrect number of shards: ", len(v.Shards))
			}
			if v.Shards[0] != 3 {
				t.Fatal("node 2 does not hold shard 3 after mirgation", v.Shards[0])
			}
		} else {
			if len(v.Shards) != 4 {
				t.Fatal("node 1 holds incorrect number of shards: ", len(v.Shards))
			}

			for _, s := range v.Shards {
				if s == 3 {
					t.Fatal("node 1 holds migrated shard")
				}
			}
		}
	}
}

func TestMigrateNode(t *testing.T) {
	orchestrator := CreateMockOrchestrator(10)
	err := orchestrator.Start(testServerAddr)
	if err != nil {
		t.Fatal("failed starting orchestrator: ", err)
		return
	}
	defer orchestrator.Stop()

	dataToMigrate := []string{
		"hello world",
		"is this working?",
		"i hope so",
	}

	dshardorchestrator.RegisterUserEvent("test", 101, "")

	// set up the sessions
	var n1 *node.Conn

	sessionWaitChan := make(chan node.SessionInfo, 10)
	shardStartedChan := make(chan int, 10)
	shardsAddedChan := make(chan []int, 10)

	dataReceived := make([]bool, len(dataToMigrate))
	var dataReceivedMU sync.Mutex

	bot1 := &MockBot{
		SessionEstablishedFunc: func(info node.SessionInfo) {
			sessionWaitChan <- info
		},
		ResumeShardFunc: func(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
			shardStartedChan <- shard
		},
		AddNewShardFunc: func(shards ...int) {
			shardsAddedChan <- shards
		},
		StartShardTransferFromFunc: func(shard int) int {

			dataReceivedMU.Lock()
			dataReceived = make([]bool, len(dataToMigrate))
			dataReceivedMU.Unlock()
			go func() {
				// take anohter path than the TestMigrateShard test
				time.Sleep(time.Second)
				for _, v := range dataToMigrate {
					n1.SendLogErr(101, v, false)
				}
			}()

			return len(dataToMigrate)
		},
	}

	bot2 := &MockBot{
		SessionEstablishedFunc: func(info node.SessionInfo) {
			sessionWaitChan <- info
		},
		ResumeShardFunc: func(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
			dataReceivedMU.Lock()
			for i, v := range dataReceived {
				if !v {
					t.Errorf("data did not get migrated migrated #%d: %v", i, dataToMigrate[i])
				}
			}
			dataReceivedMU.Unlock()
			shardStartedChan <- shard
		},
		HandleUserEventFunc: func(evt dshardorchestrator.EventType, data interface{}) {
			dataCast := *data.(*string)
			if evt != 101 {
				t.Fatal("event was not 101")
			}

			dataReceivedMU.Lock()
			for i, v := range dataToMigrate {
				if v == dataCast {
					dataReceived[i] = true
				}
			}
			dataReceivedMU.Unlock()
		},
	}

	// connect node 1
	n1, ok := startConnectNode(t, bot1, sessionWaitChan)
	defer n1.Close()

	if !ok {
		t.Fail()
		return
	}

	// connect node 2
	n2, ok := startConnectNode(t, bot2, sessionWaitChan)
	defer n2.Close()

	if !ok {
		t.Fail()
		return
	}

	// start 5 shards on node 1
	on1 := orchestrator.FindNodeByID(n1.GetIDLock())
	addWaitForShards(t, []int{0, 1, 2, 3, 4}, shardsAddedChan, on1)

	// make sure that the orcehstrator has gotten feedback that the shards have started
	time.Sleep(time.Millisecond * 250)

	err = orchestrator.MigrateFullNode(n1.GetIDLock(), n2.GetIDLock(), true)
	if err != nil {
		t.Fatal("failed performing migration: ", err.Error())
	}

	for i := 0; i < 5; i++ {
		select {
		case <-shardStartedChan:
		case <-time.After(time.Second * 5):
			t.Fatal("timed out waiting for shard to start after migration")
		}
	}

	// sleep another 250 milliseconds again to allow time for the shard orchestrator to acknowledge that its started
	time.Sleep(time.Millisecond * 250)

	// confirm the setup
	statuses := orchestrator.GetFullNodesStatus()
	for _, v := range statuses {
		if v.ID == n2.GetIDLock() {
			if len(v.Shards) != 5 {
				t.Fatal("node 2 holds incorrect number of shards: ", len(v.Shards))
			}
		} else {
			if len(v.Shards) != 0 {
				t.Fatal("node 1 holds incorrect number of shards: ", len(v.Shards))
			}
		}
	}
}

func addWaitForShards(t *testing.T, shards []int, addChan chan []int, nc *orchestrator.NodeConn) {
	// start 5 shards on node 1
	nc.StartShards(shards...)
	select {
	case addedS := <-addChan:
		if len(addedS) != len(shards) {
			t.Fatal("mismatched added shards len's ")
		}

	OUTER:
		for _, s := range shards {
			for _, vs := range addedS {
				if vs == s {
					continue OUTER
				}
			}

			// Couldn't find that shard
			t.Fatalf("Shard %d not added", s)
		}
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for shard to start")
	}
}
