package twitter

import (
	"context"
	"fmt"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/twitter/models"
)

func (p *Plugin) Status() (string, string) {
	numFeeds, err := models.TwitterFeeds(models.TwitterFeedWhere.Enabled.EQ(true)).CountG(context.Background())
	if err != nil {
		logger.WithError(err).Error("failed fetching status")
		return "Twitter feeds", "error"
	}

	return "Twitter feeds", fmt.Sprintf("%d", numFeeds)
}

func (p *Plugin) HandleMQueueError(elem *mqueue.QueuedElement, err error) {
	logger.WithError(err).Error("got error: ", err)
}
