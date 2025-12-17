package trivia

import (
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	CleanOldTriviaScores()
	ticker := time.NewTicker(1 * time.Hour)
	for {
		select {
		case <-ticker.C:
			CleanOldTriviaScores()
		case wg := <-p.stopWorkers:
			wg.Done()
			return
		}
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	p.stopWorkers <- wg
}
