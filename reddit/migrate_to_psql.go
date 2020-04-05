package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reddit/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/boil"
)

// migrateLegacyRedisFormatToPostgres migrates all feeds from all servers to postgres from the old legacy redis format
func migrateLegacyRedisFormatToPostgres() {
	common.RedisPool.Do(radix.WithConn("guild_subreddit_watch:", func(conn radix.Conn) error {
		scanner := radix.NewScanner(conn, radix.ScanOpts{
			Command: "scan",
			Pattern: "guild_subreddit_watch:*",
		})

		// san over all the keys
		var key string
		for scanner.Next(&key) {
			// retrieve the guild id from the key
			split := strings.SplitN(key, ":", 2)
			guildID, err := strconv.ParseInt(split[1], 10, 64)
			if err != nil {
				logger.WithError(err).WithField("str", key).Error("reddit: failed migrating from redis, key is invalid")
				continue
			}

			// perform the migration
			err = migrateGuildConfig(guildID)
			if err != nil {
				logger.WithError(err).WithField("str", key).Error("reddit: failed migrating from redis")
				continue
			}
			logger.Info("migrating reddit config for ", guildID)
		}

		if err := scanner.Close(); err != nil {
			logger.WithError(err).Error("failed scanning keys while migrating reddit")
			return err
		}

		return nil
	}))
}

func migrateGuildConfig(guildID int64) error {
	key := "guild_subreddit_watch:" + strconv.FormatInt(guildID, 10)

	config, err := GetLegacyConfig(key)
	if err != nil {
		return errors.WrapIff(err, "[%d].getconfig", guildID)
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		return errors.WrapIf(err, "pq.begin")
	}

	// create a new row for each feed
	for _, item := range config {
		cID, _ := strconv.ParseInt(item.Channel, 10, 64)
		if cID == 0 {
			continue
		}
		m := &models.RedditFeed{
			GuildID:   guildID,
			ChannelID: cID,
			Subreddit: strings.ToLower(item.Sub),
			UseEmbeds: item.UseEmbeds,
		}

		err := m.Insert(context.Background(), tx, boil.Infer())
		if err != nil {
			tx.Rollback()
			return errors.WrapIff(err, "[%d:%d].insert", guildID, item.ID)
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.WrapIff(err, "[%d].commit", guildID)
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "RENAME", key, "backup_"+key))
	if err != nil {
		return errors.WrapIff(err, "[%d].rename", guildID)
	}

	return nil
}

type LegacySubredditWatchItem struct {
	Sub       string `json:"sub"`
	Guild     string `json:"guild"`
	Channel   string `json:"channel"`
	ID        int    `json:"id"`
	UseEmbeds bool   `json:"use_embeds"`
}

func FindLegacyWatchItem(source []*LegacySubredditWatchItem, id int) *LegacySubredditWatchItem {
	for _, c := range source {
		if c.ID == id {
			return c
			break
		}
	}
	return nil
}

func (item *LegacySubredditWatchItem) Set() error {
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

func (item *LegacySubredditWatchItem) Remove() error {
	guild := item.Guild

	err := common.RedisPool.Do(radix.Pipeline(
		radix.FlatCmd(nil, "HDEL", "guild_subreddit_watch:"+guild, item.ID),
		radix.FlatCmd(nil, "HDEL", "global_subreddit_watch:"+strings.ToLower(item.Sub), fmt.Sprintf("%s:%d", guild, item.ID)),
	))
	return err
}

func GetLegacyConfig(key string) ([]*LegacySubredditWatchItem, error) {
	var rawItems map[string]string
	err := common.RedisPool.Do(radix.Cmd(&rawItems, "HGETALL", key))
	if err != nil {
		return nil, err
	}

	out := make([]*LegacySubredditWatchItem, len(rawItems))

	i := 0
	for k, raw := range rawItems {
		var decoded *LegacySubredditWatchItem
		err := json.Unmarshal([]byte(raw), &decoded)
		if err != nil {
			return nil, err
		}

		if err != nil {
			id, _ := strconv.ParseInt(k, 10, 32)
			out[i] = &LegacySubredditWatchItem{
				Sub:     "ERROR",
				Channel: "ERROR DECODING",
				ID:      int(id),
			}
			logger.WithError(err).Error("Failed decoding reddit watch item")
		} else {
			out[i] = decoded
		}
		i++
	}

	return out, nil
}
