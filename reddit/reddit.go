package reddit

//go:generate esc -o assets_gen.go -pkg reddit -ignore ".go" assets/

import (
	"encoding/json"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/mediocregopher/radix.v2/redis"
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
	if code != discordgo.ErrCodeUnknownChannel {
		l := log.WithError(err).WithField("channel", elem.Channel)
		if code != discordgo.ErrCodeMissingPermissions && code != discordgo.ErrCodeMissingAccess {
			l = l.WithField("s_msg", elem.MessageEmbed)
		}
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

	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis client from pool")
		return
	}
	defer common.RedisPool.Put(client)

	currentConfig, err := GetConfig(client, "guild_subreddit_watch:"+guildID)
	if err != nil {
		log.WithError(err).Error("Failed fetching config to remove")
		return
	}

	parsed, err := strconv.Atoi(itemID)
	if err != nil {
		log.WithError(err).WithField("mq_id", elem.ID).Error("Failed parsing item id")
	}
	for _, v := range currentConfig {
		if v.ID == parsed {
			v.Remove(client)
			common.AddCPLogEntry(common.BotUser, guildID, "Removed reddit feed from "+v.Sub+", Channel does not exist")
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
	Sub     string `json:"sub"`
	Guild   string `json:"guild"`
	Channel string `json:"channel"`
	ID      int    `json:"id"`
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

func (item *SubredditWatchItem) Set(client *redis.Client) error {
	serialized, err := json.Marshal(item)
	if err != nil {
		return err
	}
	guild := item.Guild

	cmds := []*common.RedisCmd{
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"guild_subreddit_watch:" + guild, item.ID, serialized}},
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"global_subreddit_watch:" + strings.ToLower(item.Sub), fmt.Sprintf("%s:%d", guild, item.ID), serialized}},
	}

	_, err = common.SafeRedisCommands(client, cmds)
	return err
}

func (item *SubredditWatchItem) Remove(client *redis.Client) error {
	guild := item.Guild
	cmds := []*common.RedisCmd{
		&common.RedisCmd{Name: "HDEL", Args: []interface{}{"guild_subreddit_watch:" + guild, item.ID}},
		&common.RedisCmd{Name: "HDEL", Args: []interface{}{"global_subreddit_watch:" + strings.ToLower(item.Sub), fmt.Sprintf("%s:%d", guild, item.ID)}},
	}
	_, err := common.SafeRedisCommands(client, cmds)
	return err
}

func GetConfig(client *redis.Client, key string) ([]*SubredditWatchItem, error) {
	rawItems, err := client.Cmd("HGETALL", key).Map()
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
