package reddit

import (
	"context"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reddit/models"
	"github.com/pkg/errors"
)

var _ bot.RemoveGuildHandler = (*Plugin)(nil)

func (p *Plugin) RemoveGuild(g int64) error {
	_, err := models.RedditFeeds(models.RedditFeedWhere.GuildID.EQ(g)).DeleteAll(context.Background(), common.PQ)
	if err != nil {
		return errors.Wrap(err, "failed removing reddit feeds")
	}

	return nil
}

// func (p *Plugin) Status() (string, string) {
// 	subs := 0
// 	channels := 0
// 	cursor := "0"

// 	common.

// 	for {
// 		reply := client.Cmd("SCAN", cursor, "MATCH", "global_subreddit_watch:*")
// 		if reply.Err != nil {
// 			logrus.WithError(reply.Err).Error("Error scanning")
// 			break
// 		}

// 		elems, err := reply.Array()
// 		if err != nil {
// 			logrus.WithError(err).Error("Error reading reply")
// 			break
// 		}

// 		if len(elems) < 2 {
// 			logrus.Error("Invalid scan")
// 			break
// 		}

// 		newCursor, err := elems[0].Str()
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed retrieving new cursor")
// 			break
// 		}
// 		cursor = newCursor

// 		list, err := elems[1].List()
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed retrieving list")
// 			break
// 		}

// 		for _, key := range list {
// 			config, err := GetConfig(key)
// 			if err != nil {
// 				logrus.WithError(err).Error("Failed reading global config")
// 				continue
// 			}
// 			if len(config) < 1 {
// 				continue
// 			}
// 			subs++
// 			channels += len(config)
// 		}

// 		if cursor == "" || cursor == "0" {
// 			break
// 		}
// 	}

// 	return "Subs/Channels", fmt.Sprintf("%d/%d", subs, channels)
// }
