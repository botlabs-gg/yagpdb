package antiphishing

import (
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	duration := time.Minute * 60
	ticker := time.NewTicker(duration)
	started := time.Now()

	// create an initial cache when the background worker starts
	domains, err := cacheAllPhishingDomains()
	if err != nil {
		logger.WithError(err).Error("[antiphishing] Failed to create a cache of Phishing domains")
	} else {
		logger.Infof("[antiphishing] Took %s to create cache of %v Phishing Domains", time.Since(started), len(domains))
	}

	// hit the update API every 60 minutes to get all changes
	for {
		select {
		case <-ticker.C:
			started = time.Now()
			added, deleted, err := updateCachedPhishingDomains(uint32(duration.Seconds()))
			if err != nil {
				logger.WithError(err).Error("[antiphishing] Failed to update the cache of Phishing domains")
			} else {
				logger.Infof("[antiphishing] Took %s to update cache of Phishing Domains. %v added, %v removed", time.Since(started), len(added), len(deleted))
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
