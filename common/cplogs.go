package common

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/mediocregopher/radix/v3"
)

type CPLogEntry struct {
	Timestamp int64  `json:"ts"`
	Action    string `json:"action"`

	TimestampString string `json:"-"`
}

func AddCPLogEntry(user *discordgo.User, guild int64, args ...interface{}) {
	action := fmt.Sprintf("(UserID: %d) %s#%s: %s", user.ID, user.Username, user.Discriminator, fmt.Sprint(args...))

	now := time.Now()
	entry := &CPLogEntry{
		Timestamp: now.Unix(),
		Action:    action,
	}

	serialized, err := json.Marshal(entry)
	if err != nil {
		logger.WithError(err).Error("Failed marshalling cp log entry")
		return
	}

	key := "cp_logs:" + discordgo.StrID(guild)
	err = RedisPool.Do(radix.Cmd(nil, "LPUSH", key, string(serialized)))
	RedisPool.Do(radix.Cmd(nil, "LTRIM", key, "0", "100"))
	if err != nil {
		logger.WithError(err).WithField("guild", guild).Error("Failed updating cp logs")
	}
}

func GetCPLogEntries(guild int64) ([]*CPLogEntry, error) {
	var entriesRaw [][]byte
	err := RedisPool.Do(radix.Cmd(&entriesRaw, "LRANGE", "cp_logs:"+discordgo.StrID(guild), "0", "-1"))
	if err != nil {
		return nil, err
	}

	result := make([]*CPLogEntry, len(entriesRaw))

	for k, entryRaw := range entriesRaw {
		var decoded *CPLogEntry
		err = json.Unmarshal(entryRaw, &decoded)
		if err != nil {
			result[k] = &CPLogEntry{Action: "Failed decoding"}
			logger.WithError(err).WithField("guild", guild).WithField("cp_log_enry", k).Error("Failed decoding cp log entry")
		} else {
			decoded.TimestampString = time.Unix(decoded.Timestamp, 0).Format(time.Stamp)
			result[k] = decoded
		}
	}
	return result, nil
}
