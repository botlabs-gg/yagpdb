package cplogs

import (
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/sirupsen/logrus"
)

type ActionFormat struct {
	Key          string
	FormatString string
	Found        bool
}

var actionFormats = make(map[string]*ActionFormat)

// RegisterActionFormat sets up a action format, call this in your package init function
func RegisterActionFormat(format *ActionFormat) string {
	format.Found = true
	actionFormats[format.Key] = format

	return format.Key
}

// AddEntry adds a entry to the database, also generating a "LocalID" for this entry
func AddEntry(entry *LogEntry) error {

	localID, err := common.GenLocalIncrIDPQ(nil, entry.GuildID, "control_panel_logs")
	if err != nil {
		return err
	}

	rawEntry := entry.toRawLogEntry()
	rawEntry.LocalID = localID

	const insertStatement = `INSERT INTO panel_logs
	(guild_id, local_id, author_id, author_username, action, param1_type, param1_int, param1_string, param2_type, param2_int, param2_string, created_at)
	VALUES (:guild_id, :local_id, :author_id, :author_username, :action, :param1_type, :param1_int, :param1_string, :param2_type, :param2_int, :param2_string, :created_at);`

	_, err = common.SQLX.NamedExec(insertStatement, rawEntry)
	return err
}

// RetryAddEntry will etry AddEntry until it suceeds or 60 seconds has elapsed
func RetryAddEntry(entry *LogEntry) {
	started := time.Now()
	for {
		err := AddEntry(entry)
		if err == nil {
			return
		}

		if time.Since(started) > time.Minute {
			logrus.WithError(err).Errorf("gave up retrying adding panel log entry, key: %s, formatted: %s", entry.Action.Key, entry.Action.String())
			return
		}
		logrus.WithError(err).Errorf("failed saving panel log entry, retrying in a second... key: %s, formatted: %s", entry.Action.Key, entry.Action.String())

		time.Sleep(time.Second)
	}
}

func GetEntries(guildID int64, limit int, before int64) ([]*LogEntry, error) {
	result := []rawLogEntry{}

	var err error
	if before > 0 {
		err = common.SQLX.Select(&result, "SELECT * FROM panel_logs WHERE guild_id=$1 AND local_id < $3 ORDER BY local_id DESC LIMIT $2", guildID, limit, before)
	} else {
		err = common.SQLX.Select(&result, "SELECT * FROM panel_logs WHERE guild_id=$1 ORDER BY local_id DESC LIMIT $2", guildID, limit)
	}

	if err != nil {
		return nil, err
	}

	parsedResult := make([]*LogEntry, 0, len(result))
	for _, v := range result {
		parsedResult = append(parsedResult, v.toLogEntry())
	}

	return parsedResult, nil
}
