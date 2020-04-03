package reddit

import (
	"context"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/go-reddit"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/feeds"
	"github.com/jonas747/yagpdb/reddit/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"golang.org/x/oauth2"
)

const (
	MaxPostsHourFast = 200
	MaxPostsHourSlow = 200
)

var (
	confClientID     = config.RegisterOption("yagpdb.reddit.clientid", "Client ID for the reddit api application", "")
	confClientSecret = config.RegisterOption("yagpdb.reddit.clientsecret", "Client Secret for the reddit api application", "")
	confRedirectURI  = config.RegisterOption("yagpdb.reddit.redirect", "Redirect URI for the reddit api application", "")
	confRefreshToken = config.RegisterOption("yagpdb.reddit.refreshtoken", "RefreshToken for the reddit api application, you need to ackquire this manually, should be set to permanent", "")

	feedLock sync.Mutex
	fastFeed *PostFetcher
	slowFeed *PostFetcher
)

func (p *Plugin) StartFeed() {
	go p.runBot()
}

func (p *Plugin) StopFeed(wg *sync.WaitGroup) {
	wg.Add(1)

	feedLock.Lock()

	if fastFeed != nil {
		ff := fastFeed
		go func() {
			ff.StopChan <- wg
		}()
		fastFeed = nil
	} else {
		wg.Done()
	}

	if slowFeed != nil {
		sf := slowFeed
		go func() {
			sf.StopChan <- wg
		}()
		slowFeed = nil
	} else {
		wg.Done()
	}

	feedLock.Unlock()
}

func UserAgent() string {
	return fmt.Sprintf("YAGPDB:%s:%s (by /u/jonas747)", confClientID.GetString(), common.VERSION)
}

func setupClient() *reddit.Client {
	authenticator := reddit.NewAuthenticator(UserAgent(), confClientID.GetString(), confClientSecret.GetString(), confRedirectURI.GetString(),
		"a", reddit.ScopeEdit+" "+reddit.ScopeRead)
	redditClient := authenticator.GetAuthClient(&oauth2.Token{RefreshToken: confRefreshToken.GetString()}, UserAgent())
	return redditClient
}

func (p *Plugin) runBot() {
	feedLock.Lock()

	if os.Getenv("YAGPDB_REDDIT_FAST_FEED_DISABLED") == "" {
		fastFeed = NewPostFetcher(p.redditClient, false, NewPostHandler(false))
		go fastFeed.Run()
	}

	slowFeed = NewPostFetcher(p.redditClient, true, NewPostHandler(true))
	go slowFeed.Run()

	feedLock.Unlock()
}

type PostHandlerImpl struct {
	Slow        bool
	ratelimiter *Ratelimiter
}

func NewPostHandler(slow bool) PostHandler {
	rl := NewRatelimiter()
	go rl.RunGCLoop()

	return &PostHandlerImpl{
		Slow:        slow,
		ratelimiter: rl,
	}
}

func (p *PostHandlerImpl) HandleRedditPosts(links []*reddit.Link) {
	for _, v := range links {
		if strings.EqualFold(v.Selftext, "[removed]") || strings.EqualFold(v.Selftext, "[deleted]") {
			continue
		}

		if !v.IsRobotIndexable {
			continue
		}

		// since := time.Since(time.Unix(int64(v.CreatedUtc), 0))
		// logger.Debugf("[%5.2fs %6s] /r/%-20s: %s", since.Seconds(), v.ID, v.Subreddit, v.Title)
		p.handlePost(v, 0)
	}
}

func (p *PostHandlerImpl) handlePost(post *reddit.Link, filterGuild int64) error {

	// createdSince := time.Since(time.Unix(int64(post.CreatedUtc), 0))
	// logger.Printf("[%5.1fs] /r/%-15s: %s, %s", createdSince.Seconds(), post.Subreddit, post.Title, post.ID)

	qms := []qm.QueryMod{
		models.RedditFeedWhere.Subreddit.EQ(strings.ToLower(post.Subreddit)),
		models.RedditFeedWhere.Slow.EQ(p.Slow),
		models.RedditFeedWhere.Disabled.EQ(false),
	}

	if filterGuild > 0 {
		qms = append(qms, models.RedditFeedWhere.GuildID.EQ(filterGuild))
	}

	config, err := models.RedditFeeds(qms...).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("failed retrieving reddit feeds for subreddit")
		return err
	}

	// Get the configs that listens to this subreddit, if any
	filteredItems := p.FilterFeeds(config, post)

	// No channels nothing to do...
	if len(filteredItems) < 1 {
		return nil
	}

	logger.WithFields(logrus.Fields{
		"num_channels": len(filteredItems),
		"subreddit":    post.Subreddit,
	}).Debug("Found matched reddit post")

	message, embed := CreatePostMessage(post)

	for _, item := range filteredItems {
		idStr := strconv.FormatInt(item.ID, 10)

		webhookUsername := "r/" + post.Subreddit + " • YAGPDB"

		qm := &mqueue.QueuedElement{
			Guild:           item.GuildID,
			Channel:         item.ChannelID,
			Source:          "reddit",
			SourceID:        idStr,
			UseWebhook:      true,
			WebhookUsername: webhookUsername,
		}

		if item.UseEmbeds {
			qm.MessageEmbed = embed
		} else {
			qm.MessageStr = message
		}

		mqueue.QueueMessage(qm)

		feeds.MetricPostedMessages.With(prometheus.Labels{"source": "reddit"}).Inc()
		go analytics.RecordActiveUnit(item.GuildID, &Plugin{}, "posted_reddit_message")
	}

	return nil
}

func (p *PostHandlerImpl) FilterFeeds(feeds []*models.RedditFeed, post *reddit.Link) []*models.RedditFeed {
	filteredItems := make([]*models.RedditFeed, 0, len(feeds))

OUTER:
	for _, c := range feeds {
		// remove duplicates
		for _, v := range filteredItems {
			if v.ChannelID == c.ChannelID {
				continue OUTER
			}
		}

		limit := MaxPostsHourFast
		if p.Slow {
			limit = MaxPostsHourSlow
		}

		// apply ratelimiting
		if !p.ratelimiter.CheckIncrement(time.Now(), c.GuildID, limit) {
			continue
		}

		if post.Over18 && c.FilterNSFW == FilterNSFWIgnore {
			// NSFW and we ignore nsfw posts
			continue
		} else if !post.Over18 && c.FilterNSFW == FilterNSFWRequire {
			// Not NSFW and we only care about nsfw posts
			continue
		}

		if p.Slow {
			if post.Score < c.MinUpvotes {
				// less than required upvotes
				continue
			}
		}

		filteredItems = append(filteredItems, c)
	}

	return filteredItems
}

func CreatePostMessage(post *reddit.Link) (string, *discordgo.MessageEmbed) {
	plainMessage := fmt.Sprintf("**%s**\n*by %s (<%s>)*\n",
		html.UnescapeString(post.Title), post.Author, "https://redd.it/"+post.ID)

	plainBody := ""
	parentSpoiler := false
	if post.IsSelf {
		plainBody = common.CutStringShort(html.UnescapeString(post.Selftext), 250)
	} else if post.CrosspostParent != "" && len(post.CrosspostParentList) > 0 {
		// Handle cross posts
		parent := post.CrosspostParentList[0]
		plainBody += "**" + html.UnescapeString(parent.Title) + "**\n"

		if parent.IsSelf {
			plainBody += common.CutStringShort(html.UnescapeString(parent.Selftext), 250)
		} else {
			plainBody += parent.URL
		}

		if parent.Spoiler {
			parentSpoiler = true
		}
	} else {
		plainBody = post.URL
	}

	if post.Spoiler || parentSpoiler {
		plainMessage += "|| " + plainBody + " ||"
	} else {
		plainMessage += plainBody
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:  "https://reddit.com/u/" + post.Author,
			Name: post.Author,
		},
		Provider: &discordgo.MessageEmbedProvider{
			Name: "Reddit",
			URL:  "https://reddit.com",
		},
		Description: "**" + html.UnescapeString(post.Title) + "**\n",
	}
	embed.URL = "https://redd.it/" + post.ID

	if post.IsSelf {
		//  Handle Self posts
		embed.Title = "New self post"
		if post.Spoiler {
			embed.Description += "|| " + common.CutStringShort(html.UnescapeString(post.Selftext), 250) + " ||"
		} else {
			embed.Description += common.CutStringShort(html.UnescapeString(post.Selftext), 250)
		}

		embed.Color = 0xc3fc7e
	} else if post.CrosspostParent != "" && len(post.CrosspostParentList) > 0 {
		//  Handle crossposts
		embed.Title = "New Crosspost"

		parent := post.CrosspostParentList[0]
		embed.Description += "**" + html.UnescapeString(parent.Title) + "**\n"
		if parent.IsSelf {
			// Cropsspost was a self post
			embed.Color = 0xc3fc7e
			if parent.Spoiler {
				embed.Description += "|| " + common.CutStringShort(html.UnescapeString(parent.Selftext), 250) + " ||"
			} else {
				embed.Description += common.CutStringShort(html.UnescapeString(parent.Selftext), 250)
			}
		} else {
			// cross post was a link most likely
			embed.Color = 0x718aed
			embed.Description += parent.URL
			if parent.Media.Type == "" && !parent.Spoiler && parent.PostHint == "image" {
				embed.Image = &discordgo.MessageEmbedImage{
					URL: parent.URL,
				}
			}
		}
	} else {
		//  Handle Link posts
		embed.Color = 0x718aed
		embed.Title = "New link post"
		embed.Description += post.URL

		if post.Media.Type == "" && !post.Spoiler && post.PostHint == "image" {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: post.URL,
			}
		}
	}

	if post.Spoiler {
		embed.Title += " [spoiler]"
	}

	plainMessage = plainMessage
	return plainMessage, embed
}

type RedditIdSlice []string

// Len is the number of elements in the collection.
func (r RedditIdSlice) Len() int {
	return len(r)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (r RedditIdSlice) Less(i, j int) bool {
	a, err1 := strconv.ParseInt(r[i], 36, 64)
	b, err2 := strconv.ParseInt(r[j], 36, 64)
	if err1 != nil {
		logger.WithError(err1).Error("Failed parsing id")
	}
	if err2 != nil {
		logger.WithError(err2).Error("Failed parsing id")
	}

	return a > b
}

// Swap swaps the elements with indexes i and j.
func (r RedditIdSlice) Swap(i, j int) {
	temp := r[i]
	r[i] = r[j]
	r[j] = temp
}
