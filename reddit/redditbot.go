package reddit

import (
	"context"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/go-reddit"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/reddit/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"html"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MaxPostsHourFast = 200
	MaxPostsHourSlow = 200
)

var (
	ClientID     = os.Getenv("YAGPDB_REDDIT_CLIENTID")
	ClientSecret = os.Getenv("YAGPDB_REDDIT_CLIENTSECRET")
	RedirectURI  = os.Getenv("YAGPDB_REDDIT_REDIRECT")
	RefreshToken = os.Getenv("YAGPDB_REDDIT_REFRESHTOKEN")

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
	return fmt.Sprintf("YAGPDB:%s:%s (by /u/jonas747)", ClientID, common.VERSIONNUMBER)
}

func setupClient() *reddit.Client {
	authenticator := reddit.NewAuthenticator(UserAgent(), ClientID, ClientSecret, RedirectURI, "a", reddit.ScopeEdit+" "+reddit.ScopeRead)
	redditClient := authenticator.GetAuthClient(&oauth2.Token{RefreshToken: RefreshToken}, UserAgent())
	return redditClient
}

func (p *Plugin) runBot() {
	redditClient := setupClient()
	feedLock.Lock()

	if os.Getenv("YAGPDB_REDDIT_FAST_FEED_DISABLED") == "" {
		fastFeed = NewPostFetcher(redditClient, false, NewPostHandler(false))
		go fastFeed.Run()
	}

	slowFeed = NewPostFetcher(redditClient, true, NewPostHandler(true))
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
		// logrus.Debugf("[%5.2fs %6s] /r/%-20s: %s", since.Seconds(), v.ID, v.Subreddit, v.Title)
		p.handlePost(v)
	}
}

func (p *PostHandlerImpl) handlePost(post *reddit.Link) error {

	// createdSince := time.Since(time.Unix(int64(post.CreatedUtc), 0))
	// logrus.Printf("[%5.1fs] /r/%-15s: %s, %s", createdSince.Seconds(), post.Subreddit, post.Title, post.ID)

	config, err := models.RedditFeeds(
		models.RedditFeedWhere.Subreddit.EQ(strings.ToLower(post.Subreddit)),
		models.RedditFeedWhere.Slow.EQ(p.Slow)).AllG(context.Background())

	if err != nil {
		logrus.WithError(err).Error("failed retrieving reddit feeds for subreddit")
		return err
	}

	// Get the configs that listens to this subreddit, if any
	filteredItems := p.FilterFeeds(config, post)

	// No channels nothing to do...
	if len(filteredItems) < 1 {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"num_channels": len(filteredItems),
		"subreddit":    post.Subreddit,
	}).Debug("Found matched reddit post")

	message, embed := CreatePostMessage(post)

	for _, item := range filteredItems {
		idStr := strconv.FormatInt(item.ID, 10)
		if item.UseEmbeds {
			mqueue.QueueMessageEmbed("reddit", idStr, item.GuildID, item.ChannelID, embed)
		} else {
			mqueue.QueueMessageString("reddit", idStr, item.GuildID, item.ChannelID, message)
		}

		if common.Statsd != nil {
			go common.Statsd.Count("yagpdb.reddit.matches", 1, []string{"subreddit:" + post.Subreddit, "guild:" + strconv.FormatInt(item.GuildID, 10)}, 1)
		}
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
	typeStr := "link"
	if post.IsSelf {
		typeStr = "self post"
	}

	plainMessage := fmt.Sprintf("<:reddit:462994034428870656> **/u/%s** posted a new %s in **/r/%s**\n**%s** - <%s>\n",
		post.Author, typeStr, post.Subreddit, html.UnescapeString(post.Title), "https://redd.it/"+post.ID)

	plainBody := ""
	if post.IsSelf {
		plainBody = common.CutStringShort(html.UnescapeString(post.Selftext), 250)
	} else {
		plainBody = post.URL
	}

	if post.Spoiler {
		plainMessage += "|| " + plainBody + " ||"
	} else {
		plainMessage += plainBody
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://reddit.com/u/" + post.Author,
			Name:    post.Author,
			IconURL: "https://" + common.Conf.Host + "/static/img/reddit_icon.png",
		},
		Provider: &discordgo.MessageEmbedProvider{
			Name: "Reddit",
			URL:  "https://reddit.com",
		},
		Description: "**" + html.UnescapeString(post.Title) + "**\n",
	}
	embed.URL = "https://redd.it/" + post.ID

	if post.IsSelf {
		embed.Title = "New self post in /r/" + post.Subreddit
		if post.Spoiler {
			embed.Description += "|| " + common.CutStringShort(html.UnescapeString(post.Selftext), 250) + " ||"
		} else {
			embed.Description += common.CutStringShort(html.UnescapeString(post.Selftext), 250)
		}

		embed.Color = 0xc3fc7e
	} else {
		embed.Color = 0x718aed
		embed.Title = "New link post in /r/" + post.Subreddit
		embed.Description += post.URL

		if post.Media.Type == "" && !post.Spoiler {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: post.URL,
			}
		}
	}

	if post.Spoiler {
		embed.Title += " [spoiler]"
	}

	plainMessage = common.EscapeSpecialMentions(plainMessage)
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
		logrus.WithError(err1).Error("Failed parsing id")
	}
	if err2 != nil {
		logrus.WithError(err2).Error("Failed parsing id")
	}

	return a > b
}

// Swap swaps the elements with indexes i and j.
func (r RedditIdSlice) Swap(i, j int) {
	temp := r[i]
	r[i] = r[j]
	r[j] = temp
}
