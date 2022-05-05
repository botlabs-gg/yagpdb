package twitter

import (
	"context"
	"fmt"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/twitter/models"
)

func (p *Plugin) Status() (string, string) {
	numFeeds, err := models.TwitterFeeds(models.TwitterFeedWhere.Enabled.EQ(true)).CountG(context.Background())
	if err != nil {
		logger.WithError(err).Error("failed fetching status")
		return "Twitter feeds", "error"
	}

	return "Twitter feeds", fmt.Sprintf("%d", numFeeds)
}

var _ mqueue.PluginWithSourceDisabler = (*Plugin)(nil)

func (p *Plugin) DisableFeed(elem *mqueue.QueuedElement, err error) {

	feedID, err := strconv.ParseInt(elem.SourceItemID, 10, 64)
	if err != nil {
		logger.WithError(err).WithField("source_id", elem.SourceItemID).Error("failed parsing sourceID!??!")
		return
	}

	_, err = models.TwitterFeeds(models.TwitterFeedWhere.ID.EQ(feedID)).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("feed_id", feedID).Error("failed removing feed")
	}
}
