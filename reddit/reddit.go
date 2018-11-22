package reddit

//go:generate esc -o assets_gen.go -pkg reddit -ignore ".go" assets/

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/premium"
	"github.com/mediocregopher/radix.v3"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
)

type Plugin struct {
	stopFeedChan chan *sync.WaitGroup
}

func (p *Plugin) Name() string {
	return "Reddit"
}

// Remove feeds if they don't point to a proper channel
func (p *Plugin) HandleMQueueError(elem *mqueue.QueuedElement, err error) {
	code, _ := common.DiscordError(err)
	if code != discordgo.ErrCodeUnknownChannel && code != discordgo.ErrCodeMissingAccess && code != discordgo.ErrCodeMissingPermissions {
		l := log.WithError(err).WithField("channel", elem.Channel)
		l = l.WithField("s_msg", elem.MessageEmbed)

		l.Warn("Error posting reddit message")
		return
	}

	log.WithError(err).WithField("channel", elem.Channel).Info("Removing reddit feed to nonexistant discord channel")

	// channelid:feed-id
	split := strings.Split(elem.SourceID, ":")
	if len(split) < 2 {
		log.Error("Invalid queued item: ", elem.ID)
		return
	}

	guildID := split[0]
	itemID := split[1]

	currentConfig, err := GetConfig("guild_subreddit_watch:" + guildID)
	if err != nil {
		log.WithError(err).Error("Failed fetching config to remove")
		return
	}

	parsed, err := strconv.Atoi(itemID)
	if err != nil {
		log.WithError(err).WithField("mq_id", elem.ID).Error("Failed parsing item id")
	}

	parsedGID, _ := strconv.ParseInt(guildID, 10, 64)
	for _, v := range currentConfig {
		if v.ID == parsed {
			v.Remove()
			common.AddCPLogEntry(common.BotUser, parsedGID, "Removed reddit feed from "+v.Sub+", Channel does not exist or no perms")
			break
		}
	}
}

func RegisterPlugin() {
	plugin := &Plugin{
		stopFeedChan: make(chan *sync.WaitGroup),
	}
	common.RegisterPlugin(plugin)
	mqueue.RegisterSource("reddit", plugin)
}

type SubredditWatchItem struct {
	Sub       string `json:"sub"`
	Guild     string `json:"guild"`
	Channel   string `json:"channel"`
	ID        int    `json:"id"`
	UseEmbeds bool   `json:"use_embeds"`
}

func FindWatchItem(source []*SubredditWatchItem, id int) *SubredditWatchItem {
	for _, c := range source {
		if c.ID == id {
			return c
			break
		}
	}
	return nil
}

func (item *SubredditWatchItem) Set() error {
	serialized, err := json.Marshal(item)
	if err != nil {
		return err
	}
	guild := item.Guild

	err = common.RedisPool.Do(radix.Pipeline(
		radix.FlatCmd(nil, "HSET", "guild_subreddit_watch:"+guild, item.ID, serialized),
		radix.FlatCmd(nil, "HSET", "global_subreddit_watch:"+strings.ToLower(item.Sub), fmt.Sprintf("%s:%d", guild, item.ID), serialized),
	))

	return err
}

func (item *SubredditWatchItem) Remove() error {
	guild := item.Guild

	err := common.RedisPool.Do(radix.Pipeline(
		radix.FlatCmd(nil, "HDEL", "guild_subreddit_watch:"+guild, item.ID),
		radix.FlatCmd(nil, "HDEL", "global_subreddit_watch:"+strings.ToLower(item.Sub), fmt.Sprintf("%s:%d", guild, item.ID)),
	))
	return err
}

func GetConfig(key string) ([]*SubredditWatchItem, error) {
	var rawItems map[string]string
	err := common.RedisPool.Do(radix.Cmd(&rawItems, "HGETALL", key))
	if err != nil {
		return nil, err
	}

	out := make([]*SubredditWatchItem, len(rawItems))

	i := 0
	for k, raw := range rawItems {
		var decoded *SubredditWatchItem
		err := json.Unmarshal([]byte(raw), &decoded)
		if err != nil {
			return nil, err
		}

		if err != nil {
			id, _ := strconv.ParseInt(k, 10, 32)
			out[i] = &SubredditWatchItem{
				Sub:     "ERROR",
				Channel: "ERROR DECODING",
				ID:      int(id),
			}
			log.WithError(err).Error("Failed decoding reddit watch item")
		} else {
			out[i] = decoded
		}
		i++
	}

	return out, nil
}

const (
	// Max feeds per guild
	GuildMaxFeedsNormal  = 100
	GuildMaxFeedsPremium = 1000
)

func MaxFeedForCtx(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return GuildMaxFeedsPremium
	}

	return GuildMaxFeedsNormal
}
