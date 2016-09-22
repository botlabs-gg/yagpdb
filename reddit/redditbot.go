package reddit

import (
	"fmt"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/common"
	"github.com/turnage/graw"
	"github.com/turnage/redditproto"
	"log"
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
	log.Println("Reddit Bot is set up!")

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
	// posted := post.GetCreatedUtc()
	//date := time.Unix(int64(posted), 0)
	//log.Println("[RedditBot] new post", date.Format(time.Stamp), ":", post.GetTitle(), "by", post.GetAuthor())

	lastPostProcessedLock.Lock()
	lastPostProcessed = time.Now()
	lastPostProcessedLock.Unlock()

	client, err := common.RedisPool.Get()
	if err != nil {
		log.Println("Failed getting connection from redis pool", err)
		return
	}
	defer common.RedisPool.Put(client)

	config, err := GetConfig(client, "global_subreddit_watch:"+strings.ToLower(post.GetSubreddit()))
	if err != nil {
		log.Println("Failed getting config from redis", err)
		return
	}

	channels := make([]string, 0)
OUTER:
	for _, c := range config {
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

	log.Println("Found post subscribed to by", len(channels), "Channels")

	author := post.GetAuthor()
	sub := post.GetSubreddit()

	typeStr := "link"
	if post.GetIsSelf() {
		typeStr = "self post"
	}

	body := fmt.Sprintf("**/u/%s Posted a new %s in /r/%s**:\n<%s>\n\n__%s__\n", author, typeStr, sub, "https://redd.it/"+post.GetId(), post.GetTitle())

	if post.GetIsSelf() {
		body += fmt.Sprintf("%s", post.GetSelftext()) + "\n\n"
	} else {
		body += post.GetUrl() + "\n\n"
	}

	log.Println("Posting a new reddit message from", sub)
	for _, channel := range channels {
		_, err := dutil.SplitSendMessage(common.BotSession, channel, body)
		if err != nil {
			log.Println("Error posting message", err)
		}
	}
}

func (b *RedditBot) Fail(err error) bool {
	errStr := err.Error()

	if strings.Index(errStr, "bad response") == 0 {
		log.Println("Bad response encountered", err)
	} else {
		log.Println("Graw encountered an unknown error", err)
	}

	return false
}

func (b *RedditBot) BlockTime() time.Duration {
	return time.Second * 10
}

func RunReddit() {
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
			log.Println("Error running graw:", err, "Retrying in a second")
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
		if needRestart {
			log.Println("RESTARTING REDDIT BOT")

			redditBotLock.Lock()
			redditBot.eng.Stop()
			redditBotLock.Unlock()

			go runBot()
			log.Println("Done restarting")
		}

	}
}
