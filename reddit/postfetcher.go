package reddit

import (
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	greddit "github.com/botlabs-gg/yagpdb/v2/lib/go-reddit"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

var KeyLastScannedPostIDFast = "reddit_last_post_id"
var KeyLastScannedPostIDSlow = "reddit_slow_last_post_id"

// PostFetcher is responsible from fetching posts from reddit at a given interval and delay
// delay being it will make sure not to call the handler on posts newer than the given delay
type PostFetcher struct {
	Name                 string
	LastScannedPostIDKey string
	LastID               int64
	StopChan             chan *sync.WaitGroup

	started     time.Time
	hasCaughtUp bool

	delay time.Duration

	redditClient *greddit.Client
	handler      PostHandler

	log *logrus.Entry
}

type PostHandler interface {
	HandleRedditPosts(links []*greddit.Link)
}

func NewPostFetcher(redditClient *greddit.Client, slow bool, handler PostHandler) *PostFetcher {
	idKey := KeyLastScannedPostIDFast
	name := "fast"
	delay := time.Minute
	if slow {
		name = "slow"
		idKey = KeyLastScannedPostIDSlow
		delay = time.Minute * 15
	}

	return &PostFetcher{
		Name:                 name,
		redditClient:         redditClient,
		LastScannedPostIDKey: idKey,
		delay:                delay,

		handler:  handler,
		log:      logger.WithField("rfeed_type", name),
		StopChan: make(chan *sync.WaitGroup),
	}
}

func (p *PostFetcher) Run() {
	lastLogged := time.Now()
	numPosts := 0

	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case wg := <-p.StopChan:
			wg.Done()
			return
		case <-ticker.C:
		}

		links, err := p.GetNewPosts()
		if err != nil {
			p.log.WithError(err).Error("error fetching new links")
			continue
		}

		lastFeedSuccessAt = time.Now()
		if len(links) < 1 {
			continue
		}

		// basic stats
		numPosts += len(links)
		if time.Since(lastLogged) >= time.Minute {
			p.log.Info("Num posts last minute: ", numPosts)
			lastLogged = time.Now()
			numPosts = 0
		}

		p.handler.HandleRedditPosts(links)
	}
}

func (p *PostFetcher) initCursor() (int64, error) {
	var storedID int64
	common.RedisPool.Do(radix.Cmd(&storedID, "GET", p.LastScannedPostIDKey))
	if storedID != 0 {
		p.log.Info("reddit feed continuing from ", storedID)
		return storedID, nil
	}

	p.log.Warn("reddit plugin failed resuming, starting from most recent post")

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

func (p *PostFetcher) GetNewPosts() ([]*greddit.Link, error) {

	if p.started.IsZero() {
		p.started = time.Now()
	}

	if p.LastID == 0 {
		lID, err := p.initCursor()
		if err != nil {
			return nil, errors.WithMessage(err, "Failed initialising cursor")
		}

		p.LastID = lID
		logrus.Info("Initialized reddit post cursor at ", lID)
	}

	toFetch := make([]string, 100)

	for i := range int64(100) {
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
		if age < p.delay {
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
		common.RedisPool.Do(radix.FlatCmd(nil, "SET", p.LastScannedPostIDKey, highestID))
	}

	if !p.hasCaughtUp {
		logrus.Info("Redditfeed processed ", len(resp), " links")
	}

	if len(resp) < 75 && !p.hasCaughtUp {
		logrus.Info("Reddit feed caught up in ", time.Since(p.started).String())
		p.hasCaughtUp = true
	}

	return resp, nil
}
