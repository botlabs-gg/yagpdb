package common

import (
	"encoding/json"
	"fmt"
	"github.com/fzzy/radix/redis"
	"log"
	"time"
)

type CPLogEntry struct {
	Timestamp int64  `json:"ts"`
	Action    string `json:"action"`

	TimestampString string `json:"-"`
}

func AddCPLogEntry(client *redis.Client, guild string, action string) {
	now := time.Now()
	entry := &CPLogEntry{
		Timestamp: now.Unix(),
		Action:    action,
	}

	serialized, err := json.Marshal(entry)
	if err != nil {
		log.Println("Failed serializing log entry", err)
		serialized = []byte(fmt.Sprintf("{\"ts\": %d, \"action\":\"Unknown (Failed serializing!)\" }", now.Unix()))
	}

	client.Append("LPUSH", "cp_logs:"+guild, serialized)
	client.Append("LTRIM", "cp_logs:"+guild, 0, 100)

	replies := GetRedisReplies(client, 2)
	for _, r := range replies {
		if r.Err != nil {
			log.Println("Failed saving log entry", err)
		}
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
			log.Println("Failed decoding cp log entry", guild, err)
		} else {
			decoded.TimestampString = time.Unix(decoded.Timestamp, 0).Format(time.Stamp)
			result[k] = decoded
		}
	}
	return result, nil
}
