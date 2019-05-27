package scheduledevents2

import (
	"context"
	"sync"
	"time"

	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*ScheduledEvents)(nil)

func (p *ScheduledEvents) RunBackgroundWorker() {
	t := time.NewTicker(time.Hour)
	for {
		n, err := models.ScheduledEvents(qm.Where("processed=true")).DeleteAll(context.Background(), common.PQ)
		if err != nil {
			logger.WithError(err).Error("error running cleanup")
		} else {
			logger.Println("cleaned up ", n, " entries")
		}

		<-t.C
	}
}
func (p *ScheduledEvents) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Done()
}
