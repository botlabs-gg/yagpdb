package reddit

import (
	"github.com/jonas747/go-reddit"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix"
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
}

func NewPostFetcher(redditClient *reddit.Client) *PostFetcher {
	return &PostFetcher{
		redditClient: redditClient,
	}
}

func (p *PostFetcher) InitCursor() (int64, error) {
	var storedID int64
	common.RedisPool.Do(radix.Cmd(&storedID, "GET", KeyLastScannedPostID))
	if storedID != 0 {
		logrus.Info("Reddit continuing from ", storedID)
		return storedID, nil
	}

	logrus.Error("Reddit plugin failed resuming, starting from a new position")

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

	end := 0
	highestID := int64(-1)
	for i, v := range resp {
		unixSeconds := int64(v.CreatedUtc)
		age := time.Since(time.Unix(unixSeconds, 0))
		// logrus.Info(age.String())

		// stay 1 minute behind
		if age.Seconds() < 60 {
			break
		}

		end = i + 1

		parsedId, err := strconv.ParseInt(v.ID, 36, 64)
		if err != nil {
			logrus.WithError(err).WithField("id", v.ID).Error("Failed parsing reddit post id")
			continue
		}

		if highestID < parsedId {
			highestID = parsedId
		}
	}

	resp = resp[:end]

	if highestID != -1 {
		p.LastID = highestID
		common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyLastScannedPostID, highestID))
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
