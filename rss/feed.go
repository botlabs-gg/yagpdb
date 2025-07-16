package rss

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"html"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/rss/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
)

const (
	PollInterval       = time.Minute * 1
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

const redisFeedsHashKey = "rss_last_item_ids"

func (p *Plugin) pollFeeds() {
	ctx := context.Background()

	subs, err := models.RSSFeedSubscriptions(
		models.RSSFeedSubscriptionWhere.Enabled.EQ(true),
	).AllG(ctx)

	if err != nil {
		logger.WithError(err).Error("Failed to load RSS feed subscriptions")
		return
	}

	logger.Infof("Polling through %d Rss feeds", len(subs))
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

func (p *Plugin) processFeed(sub *models.RSSFeedSubscription) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(sub.FeedURL)
	if err != nil {
		logger.WithError(err).WithField("url", sub.FeedURL).Warn("Failed to parse RSS feed, requesting disable via mqueue")
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

	field := fmt.Sprintf("%d", sub.ID)
	var lastPostedGUID string
	err = common.RedisPool.Do(radix.Cmd(&lastPostedGUID, "HGET", redisFeedsHashKey, field))
	if err != nil {
		logger.WithError(err).WithField("field", field).Warn("Failed to get last posted RSS item from Redis hash")
		return
	}

	var newItems []*gofeed.Item
	foundLast := lastPostedGUID == ""

	for i := len(feed.Items) - 1; i >= 0; i-- {
		item := feed.Items[i]
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if !foundLast {
			if guid == lastPostedGUID {
				foundLast = true
			}
			continue
		}
		// Ignore articles published more than 24 hours ago
		if item.PublishedParsed != nil {
			if time.Since(*item.PublishedParsed) > 24*time.Hour {
				continue
			}
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
			mentions = mentions[:len(mentions)-1]
		}

		if mentions != "" {
			container.Components = append(container.Components, discordgo.TextDisplay{Content: mentions})
		}

		var added = 0
		for _, item := range batch {
			guid := item.GUID
			if guid == "" {
				guid = item.Link
			}

			sanitizer := bluemonday.StrictPolicy()
			// Sanitize and decode title and description
			title := sanitizer.Sanitize(item.Title)
			title, _ = url.QueryUnescape(title)
			title = html.UnescapeString(title)
			if strings.TrimSpace(title) == "" {
				title = "(no title)"
			}

			desc := sanitizer.Sanitize(item.Description)
			if len(desc) > 300 {
				desc = desc[:297] + "..."
			}
			desc, _ = url.QueryUnescape(desc)
			desc = html.UnescapeString(desc)
			link := item.Link

			if link == "" {
				continue
			}
			u, err := url.ParseRequestURI(link)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				continue
			}

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
			if desc != "" {
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

		lastGUID := batch[len(batch)-1].GUID
		if lastGUID == "" {
			lastGUID = batch[len(batch)-1].Link
		}
		err = common.RedisPool.Do(radix.Cmd(nil, "HSET", redisFeedsHashKey, field, lastGUID))
		if err != nil {
			logger.WithError(err).WithField("field", field).Warn("Failed to set last posted RSS item in Redis hash")
		}
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
	logger.WithField("guild_id", guildID).Infof("Removed Excess RSS Feeds")
	_, err := models.RSSFeedSubscriptions(models.RSSFeedSubscriptionWhere.GuildID.EQ(guildID)).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("guild_id", guildID).Error("failed disabling feed for missing premium")
		return err
	}
	return nil
}

func RegisterPlugin() {
	common.InitSchemas("rss", DBSchemas...)
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
	mqueue.RegisterSource("rss", plugin)
}
