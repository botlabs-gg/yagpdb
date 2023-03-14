package mqueue

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

func TestMain(m *testing.M) {
	if err := common.InitTestRedis(); err != nil {
		fmt.Printf("Failed redis init, not running tests... %v \n", err)
		return
	}

	os.Exit(m.Run())
}

func TestMqueuePubsub(t *testing.T) {
	var wg sync.WaitGroup
	fakeProcessor := &FakeProcessor{
		retry: false,
		onHit: func(wi *workItem) {
			t.Log("hit process")
			wg.Done()
		},
	}

	// initialize
	backend := &RedisBackend{
		pool: common.RedisPool,
	}
	server := NewServer(backend, fakeProcessor)
	server.forceAllShards = true
	go server.Run()

	redisPubsub := RedisPushServer{
		pushwork:    server.PushWork,
		fullRefresh: server.refreshWork,
		selectDB:    2,
	}
	go redisPubsub.run()
	time.Sleep(time.Second)

	t.Log("init")

	// make sure it works!
	wg.Add(1)
	if err := QueueMessage(&QueuedElement{
		ChannelID:  100,
		GuildID:    10,
		Source:     "test",
		MessageStr: "test message",
	}); err != nil {
		t.Fatal(err)
	}

	t.Log("waiting for process")
	wg.Wait()

	t.Log("shutting down...")

	// shut down
	var stopWG sync.WaitGroup
	stopWG.Add(1)
	server.Stop <- &stopWG
	stopWG.Wait()
}

func TestMqueueRefresh(t *testing.T) {
	var wg sync.WaitGroup
	fakeProcessor := &FakeProcessor{
		retry: false,
		onHit: func(wi *workItem) {
			t.Log("hit process")
			wg.Done()
		},
	}

	// initialize
	backend := &RedisBackend{
		pool: common.RedisPool,
	}
	server := NewServer(backend, fakeProcessor)
	server.forceAllShards = true
	go server.Run()

	redisPubsub := RedisPushServer{
		pushwork:    server.PushWork,
		fullRefresh: server.refreshWork,
		selectDB:    2,
	}

	t.Log("init")

	// make sure it works!
	wg.Add(1)
	if err := QueueMessage(&QueuedElement{
		ChannelID:  100,
		GuildID:    10,
		Source:     "test",
		MessageStr: "test message",
	}); err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	if err := QueueMessage(&QueuedElement{
		ChannelID:  100,
		GuildID:    10,
		Source:     "test",
		MessageStr: "test message",
	}); err != nil {
		t.Fatal(err)
	}

	go redisPubsub.run()

	t.Log("waiting for process")
	wg.Wait()

	t.Log("shutting down...")

	// shut down
	var stopWG sync.WaitGroup
	stopWG.Add(1)
	server.Stop <- &stopWG
	stopWG.Wait()
}

type FakeProcessor struct {
	onHit func(wi *workItem)
	retry bool
}

func (f *FakeProcessor) ProcessItem(resp chan *workResult, wi *workItem) {
	f.onHit(wi)
	resp <- &workResult{
		item:  wi,
		retry: f.retry,
	}
}
