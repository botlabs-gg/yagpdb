package bot

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
)

const EventLoggerPeriodDuration = time.Second * 10

var (
	EventLogger = &eventLogger{}
)

type eventLogger struct {
	sync.Mutex

	totalStats [][]*int64

	lastPeriod [][]int64
	perPeriod  [][]int64

	numShards int
}

func (e *eventLogger) init(numShards int) {
	e.totalStats = make([][]*int64, numShards)
	e.lastPeriod = make([][]int64, numShards)
	e.perPeriod = make([][]int64, numShards)

	// Initialize these
	for i, _ := range e.totalStats {
		e.totalStats[i] = make([]*int64, len(eventsystem.AllEvents))
		e.lastPeriod[i] = make([]int64, len(eventsystem.AllEvents))
		e.perPeriod[i] = make([]int64, len(eventsystem.AllEvents))

		for j, _ := range e.totalStats[i] {
			e.totalStats[i][j] = new(int64)
		}
	}

}

func (e *eventLogger) run() {

	ticker := time.NewTicker(EventLoggerPeriodDuration)
	for {
		<-ticker.C
		e.flushStats()
	}
}

func (e *eventLogger) GetStats() (total [][]int64, perPeriod [][]int64) {
	e.Lock()

	total = make([][]int64, len(e.totalStats))
	perPeriod = make([][]int64, len(e.totalStats))

	for i := 0; i < len(e.totalStats); i++ {

		perPeriod[i] = make([]int64, len(e.totalStats[i]))
		total[i] = make([]int64, len(e.totalStats[i]))

		for j := 0; j < len(e.totalStats[i]); j++ {
			perPeriod[i][j] = e.perPeriod[i][j]
			total[i][j] = atomic.LoadInt64(e.totalStats[i][j])
		}
	}

	e.Unlock()

	return
}

func (e *eventLogger) flushStats() {
	totalPerPeriod := int64(0)

	e.Lock()
	for i := 0; i < len(e.totalStats); i++ {
		for j := 0; j < len(e.totalStats[i]); j++ {
			currentVal := atomic.LoadInt64(e.totalStats[i][j])
			e.perPeriod[i][j] = currentVal - e.lastPeriod[i][j]
			e.lastPeriod[i][j] = currentVal

			totalPerPeriod += e.perPeriod[i][j]
		}
	}
	e.Unlock()

	if common.Statsd != nil {
		common.Statsd.Count("discord.processed.events", totalPerPeriod, nil, EventLoggerPeriodDuration.Seconds())
	}
}

func (e *eventLogger) handleEvent(evt *eventsystem.EventData) {
	if evt.Session == nil {
		return
	}

	if int(evt.Type) >= len(eventsystem.AllEvents) {
		return
	}

	atomic.AddInt64(e.totalStats[evt.Session.ShardID][evt.Type], 1)
}
