package reddit

import (
	"encoding/json"
	"fmt"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"log"
	"strconv"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Reddit"
}

func (p *Plugin) InitWeb(rootMux *goji.Mux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/reddit.html"))

	cpMux.HandleC(pat.Get("/cp/:server/reddit"), baseData(goji.HandlerFunc(HandleReddit)))
	cpMux.HandleC(pat.Get("/cp/:server/reddit/"), baseData(goji.HandlerFunc(HandleReddit)))

	// If only html allowed patch and delete.. if only
	cpMux.HandleC(pat.Post("/cp/:server/reddit"), baseData(goji.HandlerFunc(HandleNew)))
	cpMux.HandleC(pat.Post("/cp/:server/reddit/:item/update"), baseData(goji.HandlerFunc(HandleModify)))
	cpMux.HandleC(pat.Post("/cp/:server/reddit/:item/delete"), baseData(goji.HandlerFunc(HandleRemove)))
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
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"global_subreddit_watch:" + item.Sub, fmt.Sprintf("%s:%d", guild, item.ID), serialized}},
	}

	_, err = common.SafeRedisCommands(client, cmds)
	return err
}

func (item *SubredditWatchItem) Remove(client *redis.Client) error {
	guild := item.Guild
	cmds := []*common.RedisCmd{
		&common.RedisCmd{Name: "HDEL", Args: []interface{}{"guild_subreddit_watch:" + guild, item.ID}},
		&common.RedisCmd{Name: "HDEL", Args: []interface{}{"global_subreddit_watch:" + item.Sub, fmt.Sprintf("%s:%d", guild, item.ID)}},
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
			log.Println("[Reddit]: Error decoding watch item", key, k, err)
		} else {
			out[i] = decoded
		}
		i++
	}

	return out, nil
}
