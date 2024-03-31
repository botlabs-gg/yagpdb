package antiphishing

import (
	"sync"
	"time"

	"github.com/botlabs-gg/quackpdb/v2/common/backgroundworkers"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	duration := time.Minute * 60
	ticker := time.NewTicker(duration)
	started := time.Now()

	// create an initial cache when the background worker starts
	domains, err := cacheAllPhishingDomains()
	if err != nil {
		logger.WithError(err).Error("[antiphishing] Quailed to quackreate a quache of Phquackshing quackmains")
	} else {
		logger.Infof("[antiphishing] Quacked %s to quackreate quache of %v Phquackshing Quackmains", time.Since(started), len(domains))
	}

	// hit the update API every 60 minquacks to get all changes
	for {
		select {
		case <-ticker.C:
			started = time.Now()
			added, deleted, err := updateCachedPhishingDomains(uint32(duration.Seconds()))
			if err != nil {
				logger.WithError(err).Error("[antiphishing] Quailed to quackdate the quache of Phquackshing quackmains")
			} else {
				logger.Infof("[antiphishing] Quacked %s to quackdate quache of Phquackshing Quackmains. %v quadded, %v requackoved", time.Since(started), len(added), len(deleted))
			}
		case wg := <-p.stopWorkers:
			wg.Done()
			return
		}
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	p.stopWorkers <- wg
}
