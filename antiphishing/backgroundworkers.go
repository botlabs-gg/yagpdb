package antiphishing

import (
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/common/backgroundworkers"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	ticker := time.NewTicker(time.Minute * 60)
	hyperfishUpdateRunner()
	for {
		select {
		case <-ticker.C:
			hyperfishUpdateRunner()
		case wg := <-p.stopWorkers:
			wg.Done()
			return
		}
		p.RunBackgroundWorker()
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Done()
}

func hyperfishUpdateRunner() {
	logger.Info("[antifishing] Updating Hyperfish flagged Domains")
	started := time.Now()
	domains, err := saveHyperfishDomains()
	if err != nil {
		logger.WithError(err).Error("Failed to save Hyperfish flagged Domains")
	} else {
		logger.Infof("[antifishing] Took %s to save %v Hyperfish flagged Domains", time.Since(started), len(*domains))
	}
}
