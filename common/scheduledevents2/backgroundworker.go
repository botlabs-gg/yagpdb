package scheduledevents2

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"sync"
	"time"
)

var _ common.BackgroundWorkerPlugin = (*ScheduledEvents)(nil)

func (p *ScheduledEvents) RunBackgroundWorker() {
	t := time.NewTicker(time.Hour)
	for {
		n, err := models.ScheduledEvents(qm.Where("processed=true")).DeleteAll(context.Background(), common.PQ)
		if err != nil {
			logrus.WithError(err).Error("[scheduledevents2] error running cleanup")
		} else {
			logrus.Println("[scheduledevents2] cleaned up ", n, " entries")
		}

		<-t.C
	}
}
func (p *ScheduledEvents) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Done()
}
