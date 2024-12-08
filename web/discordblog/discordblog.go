package discordblog

import (
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/shurcooL/github_flavored_markdown"
	"github.com/sirupsen/logrus"
)

type RenderedEmbed struct {
	ColorHex string
	Content  template.HTML
	ImageURL string
}

type Post struct {
	AuthorName      string
	Message         *discordgo.Message
	RenderedEmbeds  []*RenderedEmbed
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
var unixTimestampRegex = regexp.MustCompile(`(?i)<t:\d+:(t|d|f|r)>`)

func replaceUnixTimestamps(body string) string {
	return unixTimestampRegex.ReplaceAllStringFunc(body, func(match string) string {
		unix, _ := strconv.ParseInt(match[3:len(match)-3], 10, 64)
		timestamp := time.Unix(unix, 0)
		return timestamp.Format(time.RFC1123)
	})
}

func renderBody(body string) template.HTML {
	stripped := mentionStrippingRegex.ReplaceAllString(body, "")
	timestamped := replaceUnixTimestamps(stripped)
	rendered := github_flavored_markdown.Markdown([]byte(timestamped))
	return template.HTML(string(rendered))
}

func renderMessageEmbeds(msg *discordgo.Message) []*RenderedEmbed {
	var embeds []*RenderedEmbed
	for _, embed := range msg.Embeds {
		var bodyLines []string
		if embed.Author != nil {
			authorLine := embed.Author.Name
			if embed.Author.URL != "" {
				authorLine = fmt.Sprintf("[%s](%s)", authorLine, embed.Author.URL)
			}
			authorLine = "#### " + authorLine
			bodyLines = append(bodyLines, authorLine)
		}
		if embed.Title != "" {
			titleLine := embed.Title
			if embed.URL != "" {
				titleLine = fmt.Sprintf("[%s](%s)", titleLine, embed.URL)
			}
			titleLine = "### " + titleLine
			bodyLines = append(bodyLines, titleLine)
		}
		if embed.Description != "" {
			bodyLines = append(bodyLines, embed.Description)
		}
		for _, f := range embed.Fields {
			fieldBody := fmt.Sprintf("**%s**\n%s", f.Name, f.Value)
			bodyLines = append(bodyLines, fieldBody)
		}

		var footerLine string
		if embed.Footer != nil {
			footerLine = embed.Footer.Text
		}
		if embed.Timestamp != "" {
			timestamp, err := discordgo.Timestamp(embed.Timestamp).Parse()
			if err == nil {
				timestampString := timestamp.Format(time.RFC1123)
				if footerLine != "" {
					footerLine = footerLine + " · " + timestampString
				} else {
					footerLine = timestampString
				}
			}
		}
		if footerLine != "" {
			bodyLines = append(bodyLines, footerLine)
		}

		rendered := &RenderedEmbed{
			Content: renderBody(strings.Join(bodyLines, "\n\n")),
		}
		if embed.Color != 0 {
			rendered.ColorHex = fmt.Sprintf("#%06x", embed.Color)
		}
		if embed.Image != nil {
			rendered.ImageURL = embed.Image.URL
		}

		embeds = append(embeds, rendered)
	}

	return embeds
}

func formatAuthorString(author *discordgo.User) string {
	authorName := author.Globalname
	if authorName == "" {
		authorName = author.String()
	}
	return authorName
}

func updatePosts(session *discordgo.Session, channel int64) error {
	messages, err := session.ChannelMessages(channel, 25, 0, 0, 0)
	if err != nil {
		return err
	}

	postsmu.Lock()
	posts = make([]*Post, len(messages))

	for i, v := range messages {
		ts, _ := v.Timestamp.Parse()

		content := v.Content

		for _, user := range v.Mentions {
			username := formatAuthorString(user)
			content = strings.NewReplacer(
				"<@"+discordgo.StrID(user.ID)+">", "@"+username,
				"<@!"+discordgo.StrID(user.ID)+">", "@"+username,
			).Replace(content)
		}

		p := &Post{
			AuthorName:      formatAuthorString(v.Author),
			Message:         v,
			RenderedBody:    renderBody(content),
			RenderedEmbeds:  renderMessageEmbeds(v),
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
