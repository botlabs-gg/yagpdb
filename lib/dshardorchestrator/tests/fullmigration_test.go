package tests

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
)

type MockLauncher struct {
	t               *testing.T
	bot             node.Interface
	sessionWaitChan chan node.SessionInfo
}

func (ml *MockLauncher) LaunchNewNode() (string, error) {
	n, ok := startConnectNode(ml.t, ml.bot, ml.sessionWaitChan)
	if !ok {
		return "", errors.New("unable to start mock node")
	}

	return n.GetIDLock(), nil
}

func (ml *MockLauncher) LaunchVersion() (string, error) {
	return "testing", nil
}

func TestFullMigration(t *testing.T) {
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
	var n2 *node.Conn

	sessionWaitChan := make(chan node.SessionInfo, 10)
	shardStartedChan := make(chan int, 20)
	shardsAddedChan := make(chan []int, 20)

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
					if shard < 5 {
						n1.SendLogErr(101, v, false)
					} else {
						n2.SendLogErr(101, v, false)
					}
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
	n2, ok = startConnectNode(t, bot1, sessionWaitChan)
	defer n2.Close()

	if !ok {
		t.Fail()
		return
	}

	on1 := orchestrator.FindNodeByID(n1.GetIDLock())
	on2 := orchestrator.FindNodeByID(n2.GetIDLock())

	addWaitForShards(t, []int{0, 1, 2, 3, 4}, shardsAddedChan, on1)
	addWaitForShards(t, []int{5, 6, 7, 8, 9}, shardsAddedChan, on2)

	// make sure that the orcehstrator has gotten feedback that the shards have started
	time.Sleep(time.Millisecond * 250)

	orchestrator.NodeLauncher = &MockLauncher{
		bot:             bot2,
		sessionWaitChan: sessionWaitChan,
		t:               t,
	}

	err = orchestrator.MigrateAllNodesToNewNodes(true)
	if err != nil {
		t.Fatal("failed migrating: ", err)
	}

	for i := 0; i < 10; i++ {
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
			if len(v.Shards) > 0 {
				t.Fatal("node 2 holds incorrect number of shards: ", len(v.Shards))
			}
		} else if v.ID == n1.GetIDLock() {
			if len(v.Shards) > 0 {
				t.Fatal("node 1 holds incorrect number of shards: ", len(v.Shards))
			}
		} else {
			if len(v.Shards) != 5 {
				t.Fatal("node ? holds incorrect number of shards: ", len(v.Shards))
			}
		}
	}
}
