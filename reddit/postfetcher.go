package reddit

import (
	"cmp"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	greddit "github.com/botlabs-gg/yagpdb/v2/lib/go-reddit"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

var KeyLastScannedPostIDFast = "reddit_last_post_id"
var KeyLastScannedPostIDSlow = "reddit_slow_last_post_id"

const (
	fetchWindowSize        = 100
	emptyWindowsBeforeSkip = 3

	// shared by both feeds, reddit's ratelimit is per client
	catchupRequestsPerMinute = 48
)

var catchupLimiter = &requestPacer{interval: time.Minute / catchupRequestsPerMinute}

// requestPacer spaces out requests without letting idle time build into a burst.
type requestPacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (r *requestPacer) Allow(t time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t.Before(r.next) {
		return false
	}

	r.next = t.Add(r.interval)

	return true
}

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

	lastProgressUnixNano atomic.Int64
	emptyWindows         int
	behind               bool

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

	p := &PostFetcher{
		Name:                 name,
		redditClient:         redditClient,
		LastScannedPostIDKey: idKey,
		delay:                delay,

		handler:  handler,
		log:      logger.WithField("rfeed_type", name),
		StopChan: make(chan *sync.WaitGroup),
	}
	p.markProgress()

	return p
}

// waitForNextFetch reports false if the fetcher was told to stop.
func (p *PostFetcher) waitForNextFetch(ticker *time.Ticker) bool {
	if p.behind && catchupLimiter.Allow(time.Now()) {
		select {
		case wg := <-p.StopChan:
			wg.Done()
			return false
		default:
			return true
		}
	}

	select {
	case wg := <-p.StopChan:
		wg.Done()
		return false
	case <-ticker.C:
		return true
	}
}

func (p *PostFetcher) Run() {
	lastLogged := time.Now()
	numPosts := 0

	ticker := time.NewTicker(time.Second * 5)
	for p.waitForNextFetch(ticker) {
		links, err := p.GetNewPosts()
		if err != nil {
			p.log.WithError(err).Error("error fetching new links")
			continue
		}

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

func (p *PostFetcher) markProgress() {
	p.lastProgressUnixNano.Store(time.Now().UnixNano())
}

// LastProgress reports when the cursor last moved, which a successful api call alone
// does not imply.
func (p *PostFetcher) LastProgress() time.Time {
	return time.Unix(0, p.lastProgressUnixNano.Load())
}

func (p *PostFetcher) setCursor(id int64) {
	p.LastID = id
	common.RedisPool.Do(radix.FlatCmd(nil, "SET", p.LastScannedPostIDKey, id))
	p.markProgress()
}

func (p *PostFetcher) initCursor() (int64, error) {
	var storedID int64
	common.RedisPool.Do(radix.Cmd(&storedID, "GET", p.LastScannedPostIDKey))
	if storedID != 0 {
		p.log.Info("reddit feed continuing from ", storedID)
		return storedID, nil
	}

	p.log.Warn("reddit plugin failed resuming, starting from most recent post")

	return p.newestPostID()
}

func (p *PostFetcher) newestPostID() (int64, error) {
	newPosts, err := p.redditClient.GetNewLinks("all", "", "")
	if err != nil {
		return 0, err
	}

	if len(newPosts) < 1 {
		return 0, errors.New("No posts")
	}

	newest := int64(0)
	for _, v := range newPosts {
		parsed, err := strconv.ParseInt(v.ID, 36, 64)
		if err != nil {
			p.log.WithError(err).WithField("id", v.ID).Error("Failed parsing reddit post id")
			continue
		}

		newest = max(newest, parsed)
	}

	if newest == 0 {
		return 0, errors.New("No parsable post ids in /r/all/new")
	}

	return newest, nil
}

type windowPost struct {
	link *greddit.Link
	id   int64
}

func (p *PostFetcher) GetNewPosts() ([]*greddit.Link, error) {

	if p.started.IsZero() {
		p.started = time.Now()
	}

	p.behind = false

	if p.LastID == 0 {
		lID, err := p.initCursor()
		if err != nil {
			return nil, errors.WithMessage(err, "Failed initialising cursor")
		}

		p.setCursor(lID)
		p.log.Info("Initialized reddit post cursor at ", lID)
	}

	toFetch := make([]string, fetchWindowSize)

	for i := range int64(fetchWindowSize) {
		toFetch[i] = "t3_" + strconv.FormatInt(p.LastID+i+1, 36)
	}

	resp, err := p.redditClient.LinksInfo(toFetch)
	if err != nil {
		return nil, err
	}

	// /api/info does not promise ordering, and the scan below stops at the first post
	// that is too new
	posts := make([]windowPost, 0, len(resp))
	for _, v := range resp {
		parsedID, err := strconv.ParseInt(v.ID, 36, 64)
		if err != nil {
			p.log.WithError(err).WithField("id", v.ID).Error("Failed parsing reddit post id")
			continue
		}

		posts = append(posts, windowPost{link: v, id: parsedID})
	}
	slices.SortFunc(posts, func(a, b windowPost) int { return cmp.Compare(a.id, b.id) })

	links := make([]*greddit.Link, 0, len(posts))
	highestID := int64(-1)
	for _, v := range posts {
		// stay p.delay behind
		if time.Since(time.Unix(int64(v.link.CreatedUtc), 0)) < p.delay {
			break
		}

		links = append(links, v.link)
		highestID = v.id
	}

	switch {
	case highestID != -1:
		p.emptyWindows = 0
		p.setCursor(highestID)
	case len(posts) > 0:
		// too new to hand out yet
		p.emptyWindows = 0
		p.markProgress()
	default:
		if err := p.skipEmptyWindow(); err != nil {
			return nil, err
		}
	}

	if !p.hasCaughtUp {
		p.log.Info("Redditfeed processed ", len(links), " links")
	}

	p.behind = len(links) >= fetchWindowSize

	if len(links) < 75 && !p.hasCaughtUp {
		p.log.Info("Reddit feed caught up in ", time.Since(p.started).String())
		p.hasCaughtUp = true
	}

	return links, nil
}

// skipEmptyWindow steps the cursor over a run of ids reddit does not know, which would
// otherwise wedge the feed for good. Only steps once reddit is known to be past the
// window, or the cursor would run off ahead of reddit instead.
func (p *PostFetcher) skipEmptyWindow() error {
	p.emptyWindows++
	if p.emptyWindows < emptyWindowsBeforeSkip {
		return nil
	}

	newestID, err := p.newestPostID()
	if err != nil {
		return errors.WithMessage(err, "Failed checking newest post id")
	}

	windowEnd := p.LastID + fetchWindowSize
	if newestID <= windowEnd {
		p.markProgress()
		return nil
	}

	p.log.Warnf("No existing posts in id range %d-%d, skipping it (reddit is at %d)", p.LastID+1, windowEnd, newestID)
	p.setCursor(windowEnd)

	return nil
}
