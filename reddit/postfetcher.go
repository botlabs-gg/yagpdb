package reddit

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/go-reddit"
	"github.com/jonas747/yagpdb/common"
)

type PostFetcher struct {
	// Id buffer is a number of last posts, this is needed
	// because when you do `before` a post that's deleted reddit will not show anything
	// no error either (really why couldnt there be an error atleast, i mean comeon)
	// We initially try to load it from redis from a previous session
	postBuffer []string

	// healthy id offset is the current healthy id, we normally fetch "before" -1 last post so that we will always get 1 result
	// that way when we get 0 results either the post we have set our before to is deleted
	// or the post after, so we move back the id buffer untill we find a post and then update it
	// reddit api is the most complicated stupid thing ever made.
	healthyIDOffset int

	MaxPostBufferSize int

	redditClient *reddit.Client
	redisClient  *redis.Client
}

func NewPostFetcher(redditClient *reddit.Client, redisClient *redis.Client, postBuffer []string) *PostFetcher {
	if postBuffer == nil {
		postBuffer = make([]string, 0)
	}

	return &PostFetcher{
		postBuffer:        postBuffer,
		MaxPostBufferSize: 100,
		redditClient:      redditClient,
		redisClient:       redisClient,
	}
}

func (p *PostFetcher) GetNewPosts() ([]*reddit.Link, error) {

	before := ""

	if len(p.postBuffer) > 1+p.healthyIDOffset {
		before = p.postBuffer[1+p.healthyIDOffset]
		logrus.Debug("Before is", before, ", offset: ", p.healthyIDOffset)
	} else if len(p.postBuffer) > 1 {
		// ran out of id's, throw an error because this we can't recover from and we will lose posts
		// and throw away the idbuffer
		logrus.Error("Ran out of id's with id offset at ", p.healthyIDOffset)
		p.healthyIDOffset = 0
		p.postBuffer = make([]string, 0)
	}

	resp, err := p.redditClient.GetNewLinks("jonas747", "t3_"+before, "")
	if err != nil {
		return nil, err
	}

	if len(resp) < 1 {
		logrus.Info("No posts in response, incementing id offset")
		p.healthyIDOffset++
		return nil, nil
	}

	filtered := p.HandleLinksResponse(resp)
	p.SaveBuffer()
	return filtered, nil
}

// Updates the postbuffer and filters out already handled posts
func (p *PostFetcher) HandleLinksResponse(links []*reddit.Link) []*reddit.Link {

	// Filter first so that it filters against all
	filtered := make([]*reddit.Link, 0, len(links))
OUTER:
	for _, l := range links {
		for _, existing := range p.postBuffer {
			if l.ID == existing {
				continue OUTER
			}
		}

		filtered = append(filtered, l)

	}

	// Then update the postbuffer
	if p.healthyIDOffset != 0 {
		p.postBuffer = p.postBuffer[p.healthyIDOffset:]
		p.healthyIDOffset = 0
	}

	newBuffer := make([]string, 0, p.MaxPostBufferSize*2)
OUTER2:
	for _, l := range links {
		for _, existing := range p.postBuffer {
			if l.ID == existing {
				continue OUTER2
			}
		}

		newBuffer = append(newBuffer, l.ID)
	}

	newBuffer = append(newBuffer, p.postBuffer...)

	// Resize it if it exceeds the limit
	if len(newBuffer) > p.MaxPostBufferSize {
		newBuffer = newBuffer[:p.MaxPostBufferSize]
		logrus.Println("Cutting off")
	}
	p.postBuffer = newBuffer

	return filtered
}

func (p *PostFetcher) SaveBuffer() {
	err := common.SetRedisJson(p.redisClient, "reddit_last_links", p.postBuffer)
	if err != nil {
		logrus.WithError(err).Error("Failed saving post buffer")
	}
}
