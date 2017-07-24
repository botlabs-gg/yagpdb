package common

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/mediocregopher/radix.v2/redis"
	"time"
)

type CPLogEntry struct {
	Timestamp int64  `json:"ts"`
	Action    string `json:"action"`

	TimestampString string `json:"-"`
}

func AddCPLogEntry(user *discordgo.User, guild string, args ...interface{}) {
	client, err := RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection")
		return
	}
	defer RedisPool.Put(client)

	action := fmt.Sprintf("(UserID: %s) %s#%s: %s", user.ID, user.Username, user.Discriminator, fmt.Sprint(args...))

	now := time.Now()
	entry := &CPLogEntry{
		Timestamp: now.Unix(),
		Action:    action,
	}

	serialized, err := json.Marshal(entry)
	if err != nil {
		log.WithError(err).Error("Failed marshalling cp log entry")
		return
	}

	client.PipeAppend("LPUSH", "cp_logs:"+guild, serialized)
	client.PipeAppend("LTRIM", "cp_logs:"+guild, 0, 100)

	_, err = GetRedisReplies(client, 2)
	if err != nil {
		log.WithError(err).WithField("guild", guild).Error("Failed updating cp log")
	}

}

func GetCPLogEntries(client *redis.Client, guild string) ([]*CPLogEntry, error) {
	entriesRaw, err := client.Cmd("LRANGE", "cp_logs:"+guild, 0, -1).ListBytes()
	if err != nil {
		return nil, err
	}

	result := make([]*CPLogEntry, len(entriesRaw))

	for k, entryRaw := range entriesRaw {
		var decoded *CPLogEntry
		err = json.Unmarshal(entryRaw, &decoded)
		if err != nil {
			result[k] = &CPLogEntry{Action: "Failed decoding"}
			log.WithError(err).WithField("guild", guild).WithField("cp_log_enry", k).Error("Failed decoding cp log entry")
		} else {
			decoded.TimestampString = time.Unix(decoded.Timestamp, 0).Format(time.Stamp)
			result[k] = decoded
		}
	}
	return result, nil
}
