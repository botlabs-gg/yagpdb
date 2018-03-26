package reddit

import (
	"github.com/jonas747/go-reddit"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

var KeyLastScannedPostID = "reddit_last_post_id"

type PostFetcher struct {
	LastID int64

	started     time.Time
	hasCaughtUp bool

	redditClient *reddit.Client
	redisClient  *redis.Client
}

func NewPostFetcher(redditClient *reddit.Client, redisClient *redis.Client) *PostFetcher {
	return &PostFetcher{
		redditClient: redditClient,
		redisClient:  redisClient,
	}
}

func (p *PostFetcher) InitCursor() (int64, error) {
	storedId, err := p.redisClient.Cmd("GET", KeyLastScannedPostID).Int64()
	if err != nil {
		logrus.WithError(err).Error("Reddit plugin failed resuming, starting from the new position")
	} else {
		if storedId == 0 {
			logrus.Error("Reddit plugin has 0 as cursor?, starting from the new position")
		} else {
			return storedId, nil
		}
	}

	// Start from new
	newPosts, err := p.redditClient.GetNewLinks("all", "", "")
	if err != nil {
		return 0, err
	}

	if len(newPosts) < 1 {
		return 0, errors.New("No posts")
	}

	stringID := newPosts[0].ID
	parsed, err := strconv.ParseInt(stringID, 36, 64)
	return parsed, err
}

func (p *PostFetcher) GetNewPosts() ([]*reddit.Link, error) {

	if p.started.IsZero() {
		p.started = time.Now()
	}

	if p.LastID == 0 {
		lID, err := p.InitCursor()
		if err != nil {
			return nil, errors.WithMessage(err, "Failed initialising cursor")
		}

		p.LastID = lID
		logrus.Info("Initialized reddit post cursor at ", lID)
	}

	toFetch := make([]string, 100)

	for i := int64(0); i < 100; i++ {
		toFetch[i] = "t3_" + strconv.FormatInt(p.LastID+i+1, 36)
	}

	resp, err := p.redditClient.LinksInfo(toFetch)
	if err != nil {
		return nil, err
	}

	highestID := int64(-1)
	for _, v := range resp {
		parsedId, err := strconv.ParseInt(v.ID, 36, 64)
		if err != nil {
			logrus.WithError(err).WithField("id", v.ID).Error("Failed parsing reddit post id")
			continue
		}

		if highestID < parsedId {
			highestID = parsedId
		}
	}

	if highestID != -1 {
		p.LastID = highestID
		p.redisClient.Cmd("SET", KeyLastScannedPostID, highestID)
	}

	if !p.hasCaughtUp {
		logrus.Info("Redditfeed processed ", len(resp), " links")
	}

	if len(resp) < 50 && !p.hasCaughtUp {
		logrus.Info("Reddit feed caught up in ", time.Since(p.started).String())
		p.hasCaughtUp = true
	}

	return resp, nil
}
