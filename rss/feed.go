package rss

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"crypto/md5"
	"encoding/hex"

	"html"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/rss/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const (
	PollInterval       = time.Minute * 5
	maxConcurrentFeeds = 10
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct {
	Stop chan *sync.WaitGroup
}

func (p *Plugin) StartFeed() {
	p.Stop = make(chan *sync.WaitGroup)
	go p.runFeedLoop()
}

func (p *Plugin) StopFeed(wg *sync.WaitGroup) {
	if p.Stop != nil {
		p.Stop <- wg
	} else {
		wg.Done()
	}
}

func (p *Plugin) runFeedLoop() {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			p.pollFeeds()
		}
	}
}

func (p *Plugin) pollFeeds() {
	ctx := context.Background()

	subs, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
	).AllG(ctx)

	if err != nil {
		logger.WithError(err).Error("Failed to load RSS feed subscriptions")
		return
	}

	logger.Infof("Polling through %d RSS feeds", len(subs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentFeeds)

	for _, sub := range subs {
		wg.Add(1)
		sem <- struct{}{}
		go func(sub *models.RSSFeedSubscription) {
			defer wg.Done()
			defer func() { <-sem }()
			p.processFeed(sub)
		}(sub)
	}
	wg.Wait()
}

func (p *Plugin) DisableFeed(elem *mqueue.QueuedElement, err error) {
	logger.WithError(err).WithField("source_id", elem.SourceItemID).Error("Disabling RSS feed via mqueue")
	id, convErr := strconv.ParseInt(elem.SourceItemID, 10, 64)
	if convErr != nil {
		logger.WithError(convErr).WithField("source_id", elem.SourceItemID).Error("Invalid SourceItemID for disabling RSS feed")
		return
	}
	_, err = models.RSSFeedSubscriptions(models.RSSFeedSubscriptionWhere.ID.EQ(int(id))).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("feed_id", id).Error("Failed to disable RSS feed")
	}
}

// Helper to compute MD5 hash of a string
func md5Hash(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// Returns the Redis key for the seen set for a feed subscription
func seenSetKey(feedID int) string {
	return fmt.Sprintf("rss:seen:%d", feedID)
}

// Checks if the item (by URL) has already been seen for this feed
func isItemSeen(feedID int, url string) (bool, error) {
	key := seenSetKey(feedID)
	urlHash := md5Hash(url)
	var zscoreResult string
	err := common.RedisPool.Do(radix.Cmd(&zscoreResult, "ZSCORE", key, urlHash))
	if err != nil {
		return false, err
	}
	return zscoreResult != "", nil
}

func markItemSeen(feedID int, url string, published *time.Time) error {
	key := seenSetKey(feedID)
	urlHash := md5Hash(url)
	var score float64
	if published != nil {
		score = float64(published.Unix())
	} else {
		score = float64(time.Now().Unix())
	}
	if err := common.RedisPool.Do(radix.Cmd(nil, "ZADD", key, fmt.Sprintf("%f", score), urlHash)); err != nil {
		return err
	}

	return nil
}

// Cleans up items older than 90 days for this feed
func cleanupOldItems(feedID int) error {
	key := seenSetKey(feedID)
	oldest := time.Now().Add(-90 * 24 * time.Hour).Unix()
	return common.RedisPool.Do(radix.Cmd(nil, "ZREMRANGEBYSCORE", key, "0", fmt.Sprintf("%d", oldest)))
}

func getFeed(sub *models.RSSFeedSubscription, attempt int) (*gofeed.Feed, error) {
	parser := gofeed.NewParser()
	//use an http proxy if configured
	proxy := common.ConfHttpProxy.GetString()
	if len(proxy) > 0 {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			parser.Client = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				},
			}
		}
	}
	feed, err := parser.ParseURL(sub.FeedURL)
	if err != nil {
		logger.WithError(err).WithField("url", sub.FeedURL).Warnf("Failed to parse RSS feed, retrying attempt %d", attempt+1)
		if attempt < 3 {
			time.Sleep(time.Minute * time.Duration(1<<attempt))
			return getFeed(sub, attempt+1)
		}
		return nil, err
	}
	return feed, nil
}

func (p *Plugin) processFeed(sub *models.RSSFeedSubscription) {
	feed, err := getFeed(sub, 0)
	if err != nil {
		logger.WithError(err).WithField("url", sub.FeedURL).Warn("Failed to parse RSS feed, disabling feed")
		p.DisableFeed(&mqueue.QueuedElement{
			GuildID:      sub.GuildID,
			ChannelID:    sub.ChannelID,
			Source:       "rss",
			SourceItemID: strconv.Itoa(sub.ID),
		}, err)
		return
	}

	if len(feed.Items) == 0 {
		return
	}

	// We'll collect new items to post
	var newItems []*gofeed.Item
	cutoff := time.Now().Add(-24 * time.Hour)
	for i := len(feed.Items) - 1; i >= 0; i-- {
		item := feed.Items[i]
		link := item.Link
		if link == "" {
			continue
		}
		u, err := url.ParseRequestURI(link)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}

		// Use PublishedParsed, then UpdatedParsed
		var itemTime *time.Time
		if item.PublishedParsed != nil {
			itemTime = item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			itemTime = item.UpdatedParsed
		}
		if itemTime != nil && itemTime.Before(cutoff) {
			continue
		}
		item.PublishedParsed = itemTime

		seen, err := isItemSeen(sub.ID, link)
		if err != nil {
			logger.WithError(err).WithField("feed_id", sub.ID).Warn("Failed to check RSS deduplication set")
			continue
		}
		if seen {
			continue
		}
		newItems = append(newItems, item)
	}

	if len(newItems) == 0 {
		return
	}

	batchSize := 5
	for i := 0; i < len(newItems); i += batchSize {
		end := min(i+batchSize, len(newItems))
		batch := newItems[i:end]

		accentColor := 0x2b7cff
		container := discordgo.Container{
			AccentColor: accentColor,
		}

		container.Components = append(container.Components, discordgo.TextDisplay{Content: "# New Articles Published"})

		mentions := ""
		if sub.MentionEveryone {
			mentions = "@everyone"
		} else if len(sub.MentionRoles) > 0 {
			for _, roleId := range sub.MentionRoles {
				mentions += "<@&" + discordgo.StrID(roleId) + "> "
			}
			mentions = strings.TrimSpace(mentions)
		}

		if mentions != "" {
			container.Components = append(container.Components, discordgo.TextDisplay{Content: mentions})
		}

		var added = 0
		for _, item := range batch {
			link := item.Link
			if link == "" {
				continue
			}
			u, err := url.ParseRequestURI(link)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				continue
			}

			sanitizer := bluemonday.StrictPolicy()
			// Sanitize and decode title and description
			title := sanitizer.Sanitize(item.Title)
			title = html.UnescapeString(title)
			if strings.TrimSpace(title) == "" {
				title = "(no title)"
			}

			desc := sanitizer.Sanitize(item.Description)
			if len(desc) > 250 {
				desc = desc[:245] + "..."
			}
			desc = html.UnescapeString(desc)

			// Try to find an image for the post
			var imageURL string
			if item.Image != nil && item.Image.URL != "" {
				imageURL = item.Image.URL
			} else if len(item.Enclosures) > 0 {
				for _, enc := range item.Enclosures {
					if enc.Type != "" && len(enc.Type) >= 6 && enc.Type[:6] == "image/" && enc.URL != "" {
						imageURL = enc.URL
						break
					}
				}
			}
			if imageURL == "" {
				imageURL = extractImageFromMediaExtensions(item)
			}
			if imageURL == "" {
				imageURL = extractFirstImageFromHTML(item.Content)
			}
			if imageURL == "" {
				imageURL = extractFirstImageFromHTML(item.Description)
			}

			text := fmt.Sprintf("### [%s](%s)", title, link)
			if item.PublishedParsed != nil {
				text = fmt.Sprintf("%s\n-# Published <t:%d:R>\n", text, item.PublishedParsed.Unix())
			}

			if desc != "" {
				//new lines break the subtext formatting
				lines := strings.Split(desc, "\n")
				var filtered []string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						filtered = append(filtered, "-# "+line)
					}
				}
				desc = strings.Join(filtered, "\n")

				text = fmt.Sprintf("%s\n%s", text, desc)
			}

			section := discordgo.Section{
				Components: []discordgo.SectionComponentPart{},
			}

			var textDisplay discordgo.SectionComponentPart = discordgo.TextDisplay{Content: text}
			section.Components = append(section.Components, textDisplay)

			// prefer imageURL, then feed icon, then dummy RSS icon
			thumbURL := imageURL
			if thumbURL == "" && feed.Image != nil && feed.Image.URL != "" {
				thumbURL = feed.Image.URL
			}
			if thumbURL == "" {
				thumbURL = "https://upload.wikimedia.org/wikipedia/commons/6/6b/RSS_icon.jpg"
			}
			section.Accessory = discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{URL: thumbURL},
			}

			container.Components = append(container.Components, discordgo.Separator{}, section)
			added++

			// Mark as seen in Redis
			if err := markItemSeen(sub.ID, link, item.PublishedParsed); err != nil {
				logger.WithError(err).WithField("feed_id", sub.ID).Warn("Failed to mark RSS item as seen")
			}
		}
		if added == 0 {
			return
		}

		msgSend := &discordgo.MessageSend{
			Components: []discordgo.TopLevelComponent{container},
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		}

		parseMentions := []discordgo.AllowedMentionType{}
		if sub.MentionEveryone {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeEveryone)
		} else if len(sub.MentionRoles) > 0 {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles)
		}
		msgSend.AllowedMentions = discordgo.AllowedMentions{
			Parse: parseMentions,
		}

		mqueue.QueueMessage(&mqueue.QueuedElement{
			GuildID:      sub.GuildID,
			ChannelID:    sub.ChannelID,
			Source:       "rss",
			SourceItemID: strconv.Itoa(sub.ID),
			MessageSend:  msgSend,
			Priority:     2,
			AllowedMentions: discordgo.AllowedMentions{
				Parse: parseMentions,
			},
		})
	}

	// Cleanup old items after processing the feed
	if err := cleanupOldItems(sub.ID); err != nil {
		logger.WithError(err).WithField("feed_id", sub.ID).Warn("Failed to cleanup old RSS deduplication entries")
	}
}

// Helper: extract first <img src=...> from HTML
var imgSrcRegexp = regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)

func extractFirstImageFromHTML(html string) string {
	matches := imgSrcRegexp.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Helper: extract first image from media extensions (media:content, media:thumbnail, media:group)
func extractImageFromMediaExtensions(item *gofeed.Item) string {
	// Check <media:content> and <media:thumbnail> at the top level
	for _, ext := range []string{"content", "thumbnail"} {
		if mediaExts, ok := item.Extensions["media"][ext]; ok {
			for _, extVal := range mediaExts {
				if url, ok := extVal.Attrs["url"]; ok && url != "" {
					if t, ok := extVal.Attrs["type"]; !ok || (len(t) >= 6 && t[:6] == "image/") {
						return url
					}
				}
			}
		}
	}
	// Check <media:group>
	if groups, ok := item.Extensions["media"]["group"]; ok {
		for _, group := range groups {
			for _, ext := range []string{"content", "thumbnail"} {
				if children, ok := group.Children["media:"+ext]; ok {
					for _, extVal := range children {
						if url, ok := extVal.Attrs["url"]; ok && url != "" {
							if t, ok := extVal.Attrs["type"]; !ok || (len(t) >= 6 && t[:6] == "image/") {
								return url
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "RSS",
		SysName:  "rss",
		Category: common.PluginCategoryFeeds,
	}
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	logger.WithField("guild_id", guildID).Infof("Enforcing free RSS feed limits after premium removal")
	ctx := context.Background()

	toDisable, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.GuildID.EQ(guildID),
		models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
		qm.OrderBy("id DESC"),
		qm.Offset(GuildMaxRSSFeedsFree),
	).AllG(ctx)

	if err != nil {
		logger.WithError(err).WithField("guild_id", guildID).Error("failed disabling excess feeds after premium removal")
		return err
	}

	for _, feed := range toDisable {
		feed.Enabled = false
		feed.UpdateG(ctx, boil.Infer())
	}

	return nil
}

func RegisterPlugin() {
	common.InitSchemas("rss", DBSchemas...)
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
	mqueue.RegisterSource("rss", plugin)
}
