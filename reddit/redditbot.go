package reddit

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/turnage/graw"
	"github.com/turnage/redditproto"
	"strings"
	"sync"
	"time"
)

var (
	lastPostProcessed     time.Time
	lastPostProcessedLock sync.Mutex

	redditBot     *RedditBot
	redditBotLock sync.Mutex
)

type RedditBot struct {
	eng graw.Engine
}

// SetUp is a method graw looks for. If it is implemented, it will be called
// before the engine starts looking for events on Reddit. If SetUp returns an
// error, the bot will stop.
func (r *RedditBot) SetUp() error {
	r.eng = graw.GetEngine(r)

	log.Info("Reddit bot is setting up!")

	redditBotLock.Lock()
	redditBot = r
	redditBotLock.Unlock()
	return nil
}

func (r *RedditBot) TearDown() {
	redditBotLock.Lock()
	if redditBot == r {
		redditBot = nil
	}
	redditBotLock.Unlock()
}

// Called when a post is made
func (r *RedditBot) Post(post *redditproto.Link) {

	lastPostProcessedLock.Lock()
	lastPostProcessed = time.Now()
	lastPostProcessedLock.Unlock()

	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed getting connection from redis pool")
		return
	}
	defer common.RedisPool.Put(client)

	config, err := GetConfig(client, "global_subreddit_watch:"+strings.ToLower(post.GetSubreddit()))
	if err != nil {
		log.WithError(err).Error("Failed getting config from redis")
		return
	}

	channels := make([]string, 0)
OUTER:
	for _, c := range config {
		if c.Channel == "" {
			c.Channel = c.Guild
		}
		for _, currentChannel := range channels {
			if currentChannel == c.Channel {
				continue OUTER
			}
		}
		channels = append(channels, c.Channel)
	}

	if len(channels) < 1 {
		return
	}

	log.WithFields(log.Fields{
		"num_channels": len(channels),
		"subreddit":    post.GetSubreddit(),
	}).Info("Found matched reddit post")

	author := post.GetAuthor()
	sub := post.GetSubreddit()

	// typeStr := "link"
	// if post.GetIsSelf() {
	// 	typeStr = "self"
	// }

	//body := fmt.Sprintf("**/u/%s Posted a new %s in /r/%s**:\n<%s>\n\n__%s__\n", author, typeStr, sub, "https://redd.it/"+post.GetId(), post.GetTitle())
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://reddit.com/u/" + author,
			Name:    author,
			IconURL: "https://" + common.Conf.Host + "/static/img/reddit_icon.png",
		},
		Provider: &discordgo.MessageEmbedProvider{
			Name: "Reddit",
			URL:  "https://reddit.com",
		},
		Description: "**" + post.GetTitle() + "**\n",
	}
	embed.URL = "https://redd.it/" + post.GetId()

	if post.GetIsSelf() {
		embed.Title = "New self post in /r/" + sub
		embed.Description += common.CutStringShort(post.GetSelftext(), 250)
		embed.Color = 0xc3fc7e
	} else {
		embed.Color = 0x718aed
		embed.Title = "New link post in /r/" + sub
		embed.Description += post.GetUrl()
		embed.Image = &discordgo.MessageEmbedImage{
			URL: post.GetUrl(),
		}
	}

	for _, channel := range channels {
		_, err := common.BotSession.ChannelMessageSendEmbed(channel, embed)
		if err != nil {
			log.WithError(err).Error("Error posting message")
		}
	}
}

func (b *RedditBot) Fail(err error) bool {
	errStr := err.Error()

	if strings.Index(errStr, "bad response") == 0 {
		log.Error("Bad response encountered by redditt bot")
	} else {
		log.WithError(err).Error("Graw encountered an unknown error")
	}

	return false
}

func (b *RedditBot) BlockTime() time.Duration {
	return time.Second * 10
}

func (p *Plugin) StartFeed() {
	go runBot()
	monitorBot()
}

func runBot() {
	redditBotLock.Lock()
	redditBot = &RedditBot{}
	redditBotLock.Unlock()

	lastPostProcessedLock.Lock()
	lastPostProcessed = time.Now()
	lastPostProcessedLock.Unlock()

	agentFile := "reddit.agent"

	for {
		err := graw.Run(agentFile, redditBot, "all")
		if err == nil {
			break
		} else {
			log.WithError(err).Error("Error running graw")
			time.Sleep(time.Second)
		}
	}

}

func monitorBot() {
	ticker := time.NewTicker(time.Minute)
	for {
		<-ticker.C

		lastPostProcessedLock.Lock()
		needRestart := time.Since(lastPostProcessed) > time.Minute*5
		lastPostProcessedLock.Unlock()

		// Restart the bot if it has fallen asleep
		// this happens after some days of running...
		// Need to figure out the root cause
		if needRestart {
			log.Info("Restarting reddit bot!")

			redditBotLock.Lock()
			redditBot.eng.Stop()
			redditBotLock.Unlock()

			go runBot()
			log.Info("Done Restarting!")
		}

	}
}
