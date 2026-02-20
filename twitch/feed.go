package twitch

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/feeds"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/twitch/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const (
	// PollingInterval is how often we check for new streams
	PollingInterval = time.Minute * 1
)

func (p *Plugin) StartFeed() {
	p.Stop = make(chan *sync.WaitGroup)
	go p.runPoller()
}

func (p *Plugin) StopFeed(wg *sync.WaitGroup) {
	if p.Stop != nil {
		p.Stop <- wg
	} else {
		wg.Done()
	}
}

func (p *Plugin) SetupClient() error {
	clientID := confClientId.GetString()
	clientSecret := confClientSecret.GetString()

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("twitch client id or secret not set")
	}

	client, err := helix.NewClient(&helix.Options{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		return err
	}

	resp, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		return err
	}
	client.SetAppAccessToken(resp.Data.AccessToken)

	p.HelixClient = client
	return nil
}

func (p *Plugin) runPoller() {
	ticker := time.NewTicker(PollingInterval)
	for {
		select {
		case wg := <-p.Stop:
			wg.Done()
			return
		case <-ticker.C:
			p.checkStreams()
		}
	}
}

func (p *Plugin) checkStreams() {
	var items models.TwitchChannelSubscriptionSlice
	err := models.TwitchChannelSubscriptions(
		qm.Select("DISTINCT "+models.TwitchChannelSubscriptionColumns.TwitchUserID),
		models.TwitchChannelSubscriptionWhere.Enabled.EQ(true),
	).BindG(context.Background(), &items)

	if err != nil {
		logger.WithError(err).Error("Failed retrieving twitch subscriptions")
		return
	}

	var allUserIDs []string
	for _, item := range items {
		if item.TwitchUserID != "" {
			allUserIDs = append(allUserIDs, item.TwitchUserID)
		}
	}

	if len(allUserIDs) == 0 {
		return
	}

	// Process in chunks of 100
	chunks := chunkStringSlice(allUserIDs, 100)
	for i, chunk := range chunks {
		logger.Infof("Total: %d streams, Processing chunk %d/%d", len(allUserIDs), i+1, len(chunks))
		p.processStreamsBatch(chunk)
		time.Sleep(time.Second * 1)
	}
}

func (p *Plugin) processStreamsBatch(userIDs []string) {
	// Check if token is valid, refresh if needed
	err := p.RefreshAccessToken()
	if err != nil {
		logger.WithError(err).Error("Failed refreshing twitch token")
		return
	}

	// Get current stream status from Twitch
	resp, err := p.HelixClient.GetStreams(&helix.StreamsParams{
		UserIDs: userIDs,
		Type:    "live",
	})
	if err != nil {
		logger.WithError(err).Error("Failed getting streams from twitch")
		return
	}

	// Create map of currently live users
	liveMap := make(map[string]helix.Stream)
	for _, stream := range resp.Data.Streams {
		liveMap[stream.UserID] = stream
	}

	now := time.Now()
	for _, userID := range userIDs {
		stream, isLive := liveMap[userID]
		// Check if user is in the online set
		var onlineScore string
		err := common.RedisPool.Do(radix.Cmd(&onlineScore, "ZSCORE", "twitch_online_users", userID))
		wasOnline := err == nil && onlineScore != ""

		if !isLive && !wasOnline {
			continue
		}
		if isLive && wasOnline {
			// Past cooldown, update timestamp and continue (no new notification)
			common.RedisPool.Do(radix.Cmd(nil, "ZADD", "twitch_online_users", strconv.FormatInt(now.Unix(), 10), userID))
			continue
		}
		if isLive {
			common.RedisPool.Do(radix.Cmd(nil, "ZADD", "twitch_online_users", strconv.FormatInt(now.Unix(), 10), userID))
			// New stream - add to online set and notify
			logger.Infof("Twitch stream live: %s (Stream ID: %s)", stream.UserName, stream.ID)
		} else {
			// Stream went offline - remove from set and notify
			logger.Infof("Twitch stream offline: %s", userID)
			lastTime, err := strconv.ParseInt(onlineScore, 10, 64)
			if err != nil {
				logger.WithError(err).Error("Failed parsing online score")
				continue
			}
			if timeSinceOffline := now.Unix() - lastTime; timeSinceOffline < 300 {
				// 5 minutes cooldown before sending offline notification
				logger.Infof("Skipping notification for %s - only been offline for %d seconds", stream.UserName, timeSinceOffline)
				continue
			}
			common.RedisPool.Do(radix.Cmd(nil, "ZREM", "twitch_online_users", userID))
			stream = helix.Stream{
				UserID: userID,
			}
		}
		p.notifySubscribers(userID, stream, isLive)
	}
}

func chunkStringSlice(slice []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += chunkSize {
		end := min(i+chunkSize, len(slice))
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func (p *Plugin) notifySubscribers(twitchUserID string, stream helix.Stream, isLive bool) {
	userSubs, err := models.TwitchChannelSubscriptions(
		models.TwitchChannelSubscriptionWhere.TwitchUserID.EQ(twitchUserID),
		models.TwitchChannelSubscriptionWhere.Enabled.EQ(true),
	).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed retrieving twitch subscriptions for notification")
		return
	}

	if len(userSubs) == 0 {
		return
	}

	// Fill in username if missing (offline case)
	if stream.UserName == "" {
		stream.UserName = userSubs[0].TwitchUsername
		stream.UserLogin = userSubs[0].TwitchUsername
	}

	// Fetch VOD if offline and needed
	var vodUrl string
	if !isLive {
		// Check if any sub needs VOD
		needsVOD := false
		for _, sub := range userSubs {
			if sub.PublishVod {
				needsVOD = true
				break
			}
		}

		if needsVOD {
			// Fetch videos
			resp, err := p.HelixClient.GetVideos(&helix.VideosParams{
				UserID: twitchUserID,
				Type:   "archive",
				Period: "day",
				First:  1,
			})
			if err == nil && len(resp.Data.Videos) > 0 {
				// Check if video is recent (created in last 6 hours to be safe)
				video := resp.Data.Videos[0]
				created, _ := time.Parse(time.RFC3339, video.CreatedAt)
				if time.Since(created) < 6*time.Hour {
					vodUrl = video.URL
				}
			}
		}
	}

	for _, sub := range userSubs {
		p.sendStreamMessage(sub, stream, isLive, vodUrl)
	}
}

func (p *Plugin) sendStreamMessage(sub *models.TwitchChannelSubscription, stream helix.Stream, isLive bool, vodUrl string) {
	parsedGuild, _ := strconv.ParseInt(sub.GuildID, 10, 64)
	parsedChannel, _ := strconv.ParseInt(sub.ChannelID, 10, 64)

	// only send for Live events, for VODs only send if publish vod is enabled
	if !isLive && (!sub.PublishVod || vodUrl == "") {
		return
	}

	// Check for custom announcement
	announcement, err := models.FindTwitchAnnouncementG(context.Background(), parsedGuild)
	if err == nil && announcement.Enabled && len(announcement.Message) > 0 {
		// Publish event for custom announcement
		pubsub.Publish("custom_twitch_announcement", parsedGuild, CustomTwitchAnnouncement{
			GuildID:      parsedGuild,
			Subscription: *sub,
			Stream:       stream,
			IsLive:       isLive,
			VODUrl:       vodUrl,
		})
		return
	}

	var content string
	if isLive {
		streamUrl := "https://www.twitch.tv/" + stream.UserLogin
		if len(stream.GameName) > 0 {
			content = fmt.Sprintf("**%s** is live now and playing **%s**!\n%s", stream.UserName, stream.GameName, streamUrl)
		} else {
			content = fmt.Sprintf("**%s** is live now!\n%s", stream.UserName, streamUrl)
		}
	} else {
		content = fmt.Sprintf("**%s** has gone offline. Catch the VOD here: %s", stream.UserName, vodUrl)
	}

	parseMentions := []discordgo.AllowedMentionType{}

	if sub.MentionEveryone {
		content = "Hey @everyone " + content
		parseMentions = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeEveryone}
	} else if len(sub.MentionRoles) > 0 {
		mentions := "Hey"
		for _, roleId := range sub.MentionRoles {
			mentions += fmt.Sprintf(" <@&%d>", roleId)
		}
		content = mentions + " " + content
		parseMentions = []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles}
	}

	go analytics.RecordActiveUnit(parsedGuild, p, "posted_twitch_message")
	feeds.MetricPostedMessages.With(prometheus.Labels{"source": "twitch"}).Inc()

	mqueue.QueueMessage(&mqueue.QueuedElement{
		GuildID:             parsedGuild,
		ChannelID:           parsedChannel,
		Source:              "twitch",
		SourceItemID:        stream.ID,
		MessageStr:          content,
		Priority:            2,
		PublishAnnouncement: false,
		AllowedMentions: discordgo.AllowedMentions{
			Parse: parseMentions,
		},
	})
}
