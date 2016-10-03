package reddit

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"strconv"
	"strings"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Reddit"
}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
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
	rawItems, err := client.Cmd("HGETALL", key).Hash()
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
