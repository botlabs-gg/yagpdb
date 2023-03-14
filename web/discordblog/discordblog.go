package discordblog

import (
	"html/template"
	"regexp"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/shurcooL/github_flavored_markdown"
	"github.com/sirupsen/logrus"
)

type Post struct {
	Message         *discordgo.Message
	RenderedBody    template.HTML
	ParsedTimestamp time.Time
}

var (
	posts   []*Post
	postsmu sync.RWMutex
)

func RunPoller(session *discordgo.Session, channel int64, interval time.Duration) {
	logrus.Info("Starting discord blog poller on channel ", channel, " at the interval ", interval)
	ticker := time.NewTicker(interval)
	for {
		err := updatePosts(session, channel)
		if err != nil {
			logrus.WithError(err).Error("Failed polling discord blog")
		}

		<-ticker.C
	}
}

var mentionStrippingRegex = regexp.MustCompile("<@(#|&)[0-9]*>")

func updatePosts(session *discordgo.Session, channel int64) error {
	messages, err := session.ChannelMessages(channel, 25, 0, 0, 0)
	if err != nil {
		return err
	}

	postsmu.Lock()
	posts = make([]*Post, len(messages))

	for i, v := range messages {
		body := v.ContentWithMentionsReplaced()
		body = mentionStrippingRegex.ReplaceAllString(body, "")
		rendered := github_flavored_markdown.Markdown([]byte(body))

		ts, _ := v.Timestamp.Parse()

		p := &Post{
			Message:         v,
			RenderedBody:    template.HTML(string(rendered)),
			ParsedTimestamp: ts.UTC(),
		}
		posts[i] = p
	}

	postsmu.Unlock()

	return nil
}

func GetPostsAfter(after int64, limit int) []*Post {
	dest := make([]*Post, 0, limit)
	if len(posts) < 1 {
		return dest
	}

	postsmu.RLock()
	defer postsmu.RUnlock()
	for _, v := range posts {
		if v.Message.ID > after {
			dest = append(dest, v)
			if len(dest) >= limit {
				break
			}
		}
	}

	return dest
}

func GetPostsBefore(before int64, limit int) []*Post {
	dest := make([]*Post, 0, limit)
	if len(posts) < 1 {
		return dest
	}

	postsmu.RLock()
	defer postsmu.RUnlock()
	for _, v := range posts {
		if v.Message.ID < before {
			dest = append(dest, v)
			if len(dest) >= limit {
				break
			}
		}
	}

	return dest
}

func GetNewestPosts(limit int) []*Post {
	dest := make([]*Post, 0, limit)
	if len(posts) < 1 {
		return dest
	}

	postsmu.RLock()
	defer postsmu.RUnlock()

	for _, v := range posts {
		dest = append(dest, v)
		if len(dest) >= limit {
			break
		}
	}

	return dest
}
