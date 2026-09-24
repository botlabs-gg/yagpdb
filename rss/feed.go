package rss

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/rss/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const (
	PollInterval       = time.Minute * 5
	maxConcurrentFeeds = 10
	feedFetchTimeout   = 30 * time.Second

	// items older than this are never posted, so a feed that was unreachable for a
	// while doesn't dump its whole backlog once it comes back
	maxItemAge = 24 * time.Hour

	// a feed that fails for this long without a single successful fetch in between is
	// disabled. transient outages are far shorter than this, so anything that reaches
	// it is broken for good.
	feedFailureGracePeriod = 48 * time.Hour

	// how long we leave a host alone after it asks us to back off
	defaultFeedCooldown = 30 * time.Minute
	maxFeedCooldown     = 6 * time.Hour

	maxFeedSize  = 8 << 20
	feedStateTTL = 30 * 24 * time.Hour

	feedUserAgent = "YAGPDB.xyz (+https://yagpdb.xyz; https://github.com/botlabs-gg/yagpdb)"
	feedAccept    = "application/atom+xml, application/rss+xml, application/feed+json, application/xml;q=0.9, text/xml;q=0.9, */*;q=0.5"

	itemsPerMessage = 5
)

var metricFeedFetches = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "yagpdb_rss_fetches_total",
	Help: "RSS feed fetches by result",
}, []string{"result"})

var logger = common.GetPluginLogger(&Plugin{})

var descriptionSanitizer = bluemonday.StrictPolicy()

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

func (p *Plugin) Status() (string, string) {
	total, _ := models.RSSFeedSubscriptions().CountG(context.Background())
	return "Total RSS Feeds", fmt.Sprintf("%d", total)
}

func (p *Plugin) runFeedLoop() {
	ticker := time.NewTicker(PollInterval)
	p.pollFeeds()
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

	// many guilds subscribe to the same popular feeds, fetching each url once per cycle
	// keeps us off the host's rate limiter
	groups := make(map[string][]*models.RSSFeedSubscription)
	for _, sub := range subs {
		feedURL := strings.TrimSpace(sub.FeedURL)
		if feedURL == "" {
			continue
		}
		groups[feedURL] = append(groups[feedURL], sub)
	}

	logger.Infof("Polling %d RSS feeds across %d unique urls", len(subs), len(groups))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentFeeds)

	for feedURL, group := range groups {
		wg.Add(1)
		sem <- struct{}{}
		go func(feedURL string, group []*models.RSSFeedSubscription) {
			defer wg.Done()
			defer func() { <-sem }()
			// one bad feed must not take down the whole feeds process
			defer func() {
				if r := recover(); r != nil {
					logger.WithField("url", feedURL).Errorf("recovered from panic while polling rss feed\n%v\n%s", r, debug.Stack())
				}
			}()

			p.pollFeedURL(feedURL, group)
		}(feedURL, group)
	}
	wg.Wait()
}

func (p *Plugin) pollFeedURL(feedURL string, subs []*models.RSSFeedSubscription) {
	state, err := LoadFeedState(feedURL)
	if err != nil {
		logger.WithError(err).WithField("url", feedURL).Warn("Failed loading RSS feed state, fetching without it")
	}

	if state.CooldownUntil.After(time.Now()) {
		metricFeedFetches.With(prometheus.Labels{"result": "cooldown"}).Inc()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), feedFetchTimeout)
	defer cancel()

	res, err := fetchFeed(ctx, feedURL, state)
	if err != nil {
		p.handleFetchFailure(feedURL, subs, state, err)
		return
	}

	recordFetchSuccess(feedURL, res)
	if res.NotModified {
		metricFeedFetches.With(prometheus.Labels{"result": "not_modified"}).Inc()
		return
	}
	metricFeedFetches.With(prometheus.Labels{"result": "ok"}).Inc()

	items := collectPostableItems(res.Feed)
	if len(items) == 0 {
		return
	}

	for _, sub := range subs {
		p.processFeed(sub, res.Feed, items)
	}
}

// handleFetchFailure decides what a failed fetch means for the subscriptions behind it:
// back off when the host asks us to, disable right away on a permanent status, and
// otherwise give the feed feedFailureGracePeriod to recover before disabling it.
func (p *Plugin) handleFetchFailure(feedURL string, subs []*models.RSSFeedSubscription, state *feedState, err error) {
	l := logger.WithError(err).WithField("url", feedURL)
	now := time.Now()

	fErr, _ := err.(*fetchError)

	if cooldown := cooldownFor(fErr); cooldown > 0 {
		metricFeedFetches.With(prometheus.Labels{"result": "rate_limited"}).Inc()
		until := now.Add(cooldown)
		l.Warnf("RSS feed host asked us to back off, not fetching again until %s", until.UTC().Format(time.RFC1123))
		saveFeedState(feedURL, map[string]string{
			"cooldown_until": formatUnix(until),
			"last_error":     fmt.Sprintf("%s (retrying in %s)", err, common.HumanizeDuration(common.DurationPrecisionMinutes, cooldown)),
		})
		return
	}

	metricFeedFetches.With(prometheus.Labels{"result": "error"}).Inc()

	if fErr != nil && isPermanentStatus(fErr.StatusCode) {
		l.Warnf("Disabling RSS feed, the url returned %d", fErr.StatusCode)
		p.disableSubscriptions(feedURL, subs, err)
		return
	}

	firstFailure := state.FirstFailure
	if firstFailure.IsZero() {
		firstFailure = now
	}

	if now.Sub(firstFailure) >= feedFailureGracePeriod {
		l.Warnf("Disabling RSS feed, it has been failing since %s", firstFailure.UTC().Format(time.RFC1123))
		p.disableSubscriptions(feedURL, subs, err)
		return
	}

	l.Warnf("Failed fetching RSS feed, first failed %s", common.HumanizeTime(common.DurationPrecisionMinutes, firstFailure))
	saveFeedState(feedURL, map[string]string{
		"first_failure": formatUnix(firstFailure),
		"last_error":    err.Error(),
	})
}

func (p *Plugin) disableSubscriptions(feedURL string, subs []*models.RSSFeedSubscription, reason error) {
	ids := make([]int, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.ID)
	}

	if len(ids) > 0 {
		_, err := models.RSSFeedSubscriptions(
			models.RSSFeedSubscriptionWhere.ID.IN(ids),
		).UpdateAllG(context.Background(), models.M{"enabled": false, "updated_at": time.Now()})
		if err != nil {
			logger.WithError(err).WithField("url", feedURL).Error("Failed disabling RSS feed subscriptions")
			return
		}
	}

	// the failure clock is reset so re-enabling the feed gives it a fresh grace period,
	// the error is kept around so the control panel can explain what happened
	saveFeedState(feedURL, map[string]string{"last_error": fmt.Sprintf("disabled: %s", reason)}, "first_failure")
}

// the 4xx responses that can clear up on their own, so they get the same grace period
// as a 5xx instead of disabling the feed
var transientClientStatuses = []int{
	http.StatusRequestTimeout, // the origin gave up waiting on us, not a broken feed
	http.StatusTooEarly,       // asked to replay the request later
	http.StatusTooManyRequests,
}

// isPermanentStatus reports whether a status means the feed is broken rather than
// having a bad day. the rest of the 4xx range says the request itself is wrong, and
// repeating it every 5 minutes won't make it right.
func isPermanentStatus(status int) bool {
	return status >= 400 && status < 500 && !slices.Contains(transientClientStatuses, status)
}

// cooldownFor returns how long to leave a host alone after it pushed back, or 0 if it didn't
func cooldownFor(err *fetchError) time.Duration {
	if err == nil {
		return 0
	}
	if err.StatusCode != http.StatusTooManyRequests && err.StatusCode != http.StatusServiceUnavailable {
		return 0
	}

	cooldown := err.RetryAfter
	if cooldown <= 0 {
		if err.StatusCode != http.StatusTooManyRequests {
			return 0
		}
		cooldown = defaultFeedCooldown
	}

	return min(cooldown, maxFeedCooldown)
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

func itemScore(published *time.Time) int64 {
	if published != nil {
		return published.Unix()
	}
	return time.Now().Unix()
}

// claimItem marks an item as seen and reports whether this caller was the one that
// claimed it. the add is atomic so two pollers can never post the same item twice.
func claimItem(feedID int, link string, published *time.Time) (bool, error) {
	var added int
	err := common.RedisPool.Do(radix.FlatCmd(&added, "ZADD", seenSetKey(feedID), "NX", itemScore(published), md5Hash(link)))
	return added == 1, err
}

// releaseItems undoes claimItem for items we ended up not posting, so they get
// another chance on the next poll instead of being silently dropped
func releaseItems(feedID int, links []string) {
	if len(links) == 0 {
		return
	}

	args := make([]string, 0, len(links)+1)
	args = append(args, seenSetKey(feedID))
	for _, link := range links {
		args = append(args, md5Hash(link))
	}

	if err := common.RedisPool.Do(radix.Cmd(nil, "ZREM", args...)); err != nil {
		logger.WithError(err).WithField("feed_id", feedID).Warn("Failed releasing unposted RSS items")
	}
}

// SeedSeenItems marks everything currently in the feed as seen, so a newly added
// subscription starts quiet instead of posting the existing backlog
func SeedSeenItems(feedID int, feed *gofeed.Feed) error {
	args := make([]string, 0, len(feed.Items)*2+1)
	args = append(args, seenSetKey(feedID))
	for _, item := range feed.Items {
		if !isPostableLink(item.Link) {
			continue
		}
		args = append(args, strconv.FormatInt(itemScore(publishedAt(item)), 10), md5Hash(item.Link))
	}

	if len(args) == 1 {
		return nil
	}

	return common.RedisPool.Do(radix.Cmd(nil, "ZADD", args...))
}

// Cleans up items older than 90 days for this feed
func cleanupOldItems(feedID int) error {
	key := seenSetKey(feedID)
	oldest := time.Now().Add(-90 * 24 * time.Hour).Unix()
	return common.RedisPool.Do(radix.Cmd(nil, "ZREMRANGEBYSCORE", key, "0", fmt.Sprintf("%d", oldest)))
}

func (p *Plugin) processFeed(sub *models.RSSFeedSubscription, feed *gofeed.Feed, items []*feedItem) {
	posted := 0

	for i := 0; i < len(items); i += itemsPerMessage {
		batch := items[i:min(i+itemsPerMessage, len(items))]

		container, claimed := buildFeedContainer(sub, feed, batch)
		if len(claimed) == 0 {
			continue
		}

		parseMentions := []discordgo.AllowedMentionType{}
		if sub.MentionEveryone {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeEveryone)
		} else if len(sub.MentionRoles) > 0 {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles)
		}

		msgSend := &discordgo.MessageSend{
			Components:      []discordgo.TopLevelComponent{container},
			Flags:           discordgo.MessageFlagsIsComponentsV2,
			AllowedMentions: discordgo.AllowedMentions{Parse: parseMentions},
		}

		err := mqueue.QueueMessage(&mqueue.QueuedElement{
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
		if err != nil {
			logger.WithError(err).WithField("feed_id", sub.ID).Error("Failed queueing RSS message")
			releaseItems(sub.ID, claimed)
			continue
		}

		feeds.MetricPostedMessages.With(prometheus.Labels{"source": "rss"}).Inc()
		posted += len(claimed)
	}

	if posted == 0 {
		return
	}

	logger.Infof("Posted %d new items for feed %s", posted, sub.FeedURL)

	if err := cleanupOldItems(sub.ID); err != nil {
		logger.WithError(err).WithField("feed_id", sub.ID).Warn("Failed to cleanup old RSS deduplication entries")
	}
}

// buildFeedContainer renders a batch of items and returns the links it claimed for this
// subscription. items already claimed by an earlier poll are skipped.
func buildFeedContainer(sub *models.RSSFeedSubscription, feed *gofeed.Feed, batch []*feedItem) (discordgo.Container, []string) {
	container := discordgo.Container{
		AccentColor: 0x2b7cff,
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

	claimed := make([]string, 0, len(batch))
	for _, item := range batch {
		claimedItem, err := claimItem(sub.ID, item.Link, item.PublishedAt)
		if err != nil {
			logger.WithError(err).WithField("feed_id", sub.ID).Warn("Failed to check RSS deduplication set")
			continue
		}
		if !claimedItem {
			continue
		}

		container.Components = append(container.Components, discordgo.Separator{}, buildItemSection(feed, item))
		claimed = append(claimed, item.Link)
	}

	return container, claimed
}

func buildItemSection(feed *gofeed.Feed, item *feedItem) discordgo.Section {
	title := html.UnescapeString(descriptionSanitizer.Sanitize(item.Title))
	if strings.TrimSpace(title) == "" {
		title = "(no title)"
	}

	desc := html.UnescapeString(common.CutStringShort(descriptionSanitizer.Sanitize(item.Description), 250))

	text := fmt.Sprintf("### [%s](%s)", title, item.Link)
	if item.PublishedAt != nil {
		text = fmt.Sprintf("%s\n-# Published <t:%d:R>\n", text, item.PublishedAt.Unix())
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
		Components: []discordgo.SectionComponentPart{discordgo.TextDisplay{Content: text}},
	}

	// prefer the item image, then the feed icon, then a dummy RSS icon
	thumbURL := itemImageURL(item.Item)
	if thumbURL == "" && feed.Image != nil && feed.Image.URL != "" {
		thumbURL = feed.Image.URL
	}
	if thumbURL == "" {
		thumbURL = "https://upload.wikimedia.org/wikipedia/commons/6/6b/RSS_icon.jpg"
	}
	section.Accessory = discordgo.Thumbnail{
		Media: discordgo.UnfurledMediaItem{URL: thumbURL},
	}

	return section
}

func itemImageURL(item *gofeed.Item) string {
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}

	for _, enc := range item.Enclosures {
		if strings.HasPrefix(enc.Type, "image/") && enc.URL != "" {
			return enc.URL
		}
	}

	if url := extractImageFromMediaExtensions(item); url != "" {
		return url
	}
	if url := extractFirstImageFromHTML(item.Content); url != "" {
		return url
	}
	return extractFirstImageFromHTML(item.Description)
}

// feedItem is a feed entry that passed the link and age checks, with its publish time
// resolved once so every subscriber of the same url reuses it
type feedItem struct {
	*gofeed.Item
	PublishedAt *time.Time
}

func publishedAt(item *gofeed.Item) *time.Time {
	if item.PublishedParsed != nil {
		return item.PublishedParsed
	}
	return item.UpdatedParsed
}

func isPostableLink(link string) bool {
	if link == "" {
		return false
	}
	u, err := url.ParseRequestURI(link)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// collectPostableItems returns the items worth posting, oldest first
func collectPostableItems(feed *gofeed.Feed) []*feedItem {
	cutoff := time.Now().Add(-maxItemAge)

	items := make([]*feedItem, 0, len(feed.Items))
	for i := len(feed.Items) - 1; i >= 0; i-- {
		item := feed.Items[i]
		if !isPostableLink(item.Link) {
			continue
		}

		published := publishedAt(item)
		if published != nil && published.Before(cutoff) {
			continue
		}

		items = append(items, &feedItem{Item: item, PublishedAt: published})
	}

	return items
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
					if t, ok := extVal.Attrs["type"]; !ok || strings.HasPrefix(t, "image/") {
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
							if t, ok := extVal.Attrs["type"]; !ok || strings.HasPrefix(t, "image/") {
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
