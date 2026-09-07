package reddit

import (
	"sync"
	"testing"
	"time"
)

// stopFeedWaits mimics feeds.Stop, reporting whether the waitgroup ever drained.
func stopFeedWaits(t *testing.T, p *Plugin) bool {
	t.Helper()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	p.StopFeed(wg)

	drained := make(chan struct{})
	go func() {
		wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		return true
	case <-time.After(time.Second * 5):
		return false
	}
}

// answerStopSignals stands in for the goroutines StopFeed signals.
func answerStopSignals(chans ...chan *sync.WaitGroup) {
	for _, c := range chans {
		go func() {
			wg := <-c
			wg.Done()
		}()
	}
}

func TestStopFeed(t *testing.T) {
	t.Cleanup(func() {
		fastFeed = nil
		slowFeed = nil
	})

	p := &Plugin{stopFeedChan: make(chan *sync.WaitGroup)}
	fastFeed = &PostFetcher{Name: "fast", StopChan: make(chan *sync.WaitGroup)}
	slowFeed = &PostFetcher{Name: "slow", StopChan: make(chan *sync.WaitGroup)}
	answerStopSignals(fastFeed.StopChan, slowFeed.StopChan, p.stopFeedChan)

	if !stopFeedWaits(t, p) {
		t.Fatal("shutdown waitgroup never drained, the process would hang on stop")
	}

	if fastFeed != nil || slowFeed != nil {
		t.Error("fetchers should be cleared once stopped")
	}
}

func TestStopFeedWithFastFeedDisabled(t *testing.T) {
	t.Cleanup(func() { slowFeed = nil })

	p := &Plugin{stopFeedChan: make(chan *sync.WaitGroup)}
	fastFeed = nil
	slowFeed = &PostFetcher{Name: "slow", StopChan: make(chan *sync.WaitGroup)}
	answerStopSignals(slowFeed.StopChan, p.stopFeedChan)

	if !stopFeedWaits(t, p) {
		t.Fatal("shutdown waitgroup never drained, the process would hang on stop")
	}
}
