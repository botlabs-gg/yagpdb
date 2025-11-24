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
)

const (
	// PollingInterval is how often we check for new streams
	PollingInterval = time.Minute * 5
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
	// 1. Check for NEW streams (users not currently marked as online)
	p.checkNewStreams()

	// 2. Check for OFFLINE streams (users currently marked as online)
	p.checkOfflineStreams()
}

func (p *Plugin) checkNewStreams() {
	// Get all active subscriptions
	subs, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.Enabled.EQ(true)).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed retrieving twitch subscriptions")
		return
	}

	if len(subs) == 0 {
		return
	}

	// Get currently online users from Redis ZSET
	var onlineUserIDs []string
	err = common.RedisPool.Do(radix.Cmd(&onlineUserIDs, "ZRANGE", "twitch_online_users", "0", "-1"))
	if err != nil {
		logger.WithError(err).Error("Failed retrieving online users")
		return
	}

	// Create a map for faster lookup
	onlineMap := make(map[string]bool)
	for _, id := range onlineUserIDs {
		onlineMap[id] = true
	}

	// Filter out users who are already online
	uniqueUsers := make(map[string]bool)
	var usersToCheck []string

	for _, sub := range subs {
		if sub.TwitchUserID == "" {
			continue
		}
		if _, ok := uniqueUsers[sub.TwitchUserID]; !ok {
			// Only check if NOT already online
			if !onlineMap[sub.TwitchUserID] {
				uniqueUsers[sub.TwitchUserID] = true
				usersToCheck = append(usersToCheck, sub.TwitchUserID)
			}
		}
	}

	if len(usersToCheck) == 0 {
		return
	}

	// Batch requests to Twitch
	chunks := chunkStringSlice(usersToCheck, 100)
	for _, chunk := range chunks {
		p.processNewStreamsBatch(chunk, subs)
		time.Sleep(time.Second * 1)
	}
}

func (p *Plugin) processNewStreamsBatch(userIDs []string, allSubs models.TwitchChannelSubscriptionSlice) {
	// Check if token is valid, refresh if needed
	isValid, _, _ := p.HelixClient.ValidateToken(p.HelixClient.GetAppAccessToken())
	if !isValid {
		resp, err := p.HelixClient.RequestAppAccessToken([]string{})
		if err != nil {
			logger.WithError(err).Error("Failed refreshing twitch token")
			return
		}
		p.HelixClient.SetAppAccessToken(resp.Data.AccessToken)
	}

	resp, err := p.HelixClient.GetStreams(&helix.StreamsParams{
		UserIDs: userIDs,
		Type:    "live",
	})
	if err != nil {
		logger.WithError(err).Error("Failed getting streams from twitch")
		return
	}

	now := time.Now()
	for _, stream := range resp.Data.Streams {
		// Check if user is already in the online set (cooldown check)
		var lastOnlineTime string
		err := common.RedisPool.Do(radix.Cmd(&lastOnlineTime, "ZSCORE", "twitch_online_users", stream.UserID))
		if err == nil && lastOnlineTime != "" {
			// User is already marked as online, check how long ago
			lastTime, parseErr := strconv.ParseInt(lastOnlineTime, 10, 64)
			if parseErr == nil {
				timeSinceOnline := now.Unix() - lastTime
				if timeSinceOnline < 300 { // 5 minutes = 300 seconds
					logger.Infof("Skipping notification for %s - already online for %d seconds", stream.UserName, timeSinceOnline)
					continue
				}
			}
		}

		// New stream found!
		logger.Infof("Twitch stream live: %s (Stream ID: %s)", stream.UserName, stream.ID)

		// Add to Online ZSET with current timestamp
		common.RedisPool.Do(radix.Cmd(nil, "ZADD", "twitch_online_users", strconv.FormatInt(now.Unix(), 10), stream.UserID))

		// Notify
		p.notifySubscribers(stream.UserID, stream, allSubs, true)
	}
}

func (p *Plugin) checkOfflineStreams() {
	// Get currently online users
	var onlineUserIDs []string
	err := common.RedisPool.Do(radix.Cmd(&onlineUserIDs, "ZRANGE", "twitch_online_users", "0", "-1"))
	if err != nil {
		logger.WithError(err).Error("Failed retrieving online users")
		return
	}

	if len(onlineUserIDs) == 0 {
		return
	}

	// Get all subs to have context for notifications
	subs, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.Enabled.EQ(true)).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed retrieving twitch subscriptions")
		return
	}

	// Batch requests
	chunks := chunkStringSlice(onlineUserIDs, 100)
	for _, chunk := range chunks {
		p.processOfflineStreamsBatch(chunk, subs)
		time.Sleep(time.Second * 1)
	}
}

func (p *Plugin) processOfflineStreamsBatch(userIDs []string, allSubs models.TwitchChannelSubscriptionSlice) {
	// Check if token is valid, refresh if needed
	isValid, _, _ := p.HelixClient.ValidateToken(p.HelixClient.GetAppAccessToken())
	if !isValid {
		resp, err := p.HelixClient.RequestAppAccessToken([]string{})
		if err != nil {
			logger.WithError(err).Error("Failed refreshing twitch token")
			return
		}
		p.HelixClient.SetAppAccessToken(resp.Data.AccessToken)
	}

	resp, err := p.HelixClient.GetStreams(&helix.StreamsParams{
		UserIDs: userIDs,
		Type:    "live",
	})
	if err != nil {
		logger.WithError(err).Error("Failed getting streams from twitch")
		return
	}

	// Create map of currently live users from response
	liveMap := make(map[string]helix.Stream)
	for _, stream := range resp.Data.Streams {
		liveMap[stream.UserID] = stream
	}

	now := time.Now()
	for _, userID := range userIDs {
		if _, ok := liveMap[userID]; ok {
			// Still online, update heartbeat
			common.RedisPool.Do(radix.Cmd(nil, "ZADD", "twitch_online_users", strconv.FormatInt(now.Unix(), 10), userID))
		} else {
			// Offline!
			logger.Infof("Twitch stream offline: %s", userID)

			// Remove from ZSET
			common.RedisPool.Do(radix.Cmd(nil, "ZREM", "twitch_online_users", userID))

			// Notify Offline
			var username string
			for _, s := range allSubs {
				if s.TwitchUserID == userID {
					username = s.TwitchUsername
					break
				}
			}

			offlineStream := helix.Stream{
				UserID:    userID,
				UserLogin: username,
				UserName:  username,
			}
			p.notifySubscribers(userID, offlineStream, allSubs, false)
		}
	}
}

func chunkStringSlice(slice []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func (p *Plugin) notifySubscribers(twitchUserID string, stream helix.Stream, allSubs models.TwitchChannelSubscriptionSlice, isLive bool) {
	// Fetch VOD if offline and needed
	var vodUrl string
	if !isLive {
		// Check if any sub needs VOD
		needsVOD := false
		for _, sub := range allSubs {
			if sub.TwitchUserID == twitchUserID && sub.Enabled && sub.PublishVod {
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

	for _, sub := range allSubs {
		if sub.TwitchUserID == twitchUserID && sub.Enabled {
			p.sendStreamMessage(sub, stream, isLive, vodUrl)
		}
	}
}

func (p *Plugin) sendStreamMessage(sub *models.TwitchChannelSubscription, stream helix.Stream, isLive bool, vodUrl string) {
	parsedGuild, _ := strconv.ParseInt(sub.GuildID, 10, 64)
	parsedChannel, _ := strconv.ParseInt(sub.ChannelID, 10, 64)

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

	// If standard message, only send for Live events unless VOD is present
	if !isLive && (!sub.PublishVod || vodUrl == "") {
		return
	}

	var content string
	if isLive {
		streamUrl := "https://www.twitch.tv/" + stream.UserLogin
		content = fmt.Sprintf("**%s** is live now playing **%s**!\n%s", stream.UserName, stream.GameName, streamUrl)
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
