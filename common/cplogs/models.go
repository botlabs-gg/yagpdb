package cplogs

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

const DBSchema = `
CREATE TABLE IF NOT EXISTS panel_logs (
	guild_id BIGINT NOT NULL,
	local_id BIGINT NOT NULL,

	author_id BIGINT NOT NULL,
	author_username TEXT NOT NULL,

	action TEXT NOT NULL,

	param1_type SMALLINT NOT NULL,
	param1_int BIGINT NOT NULL,
	param1_string TEXT NOT NULL,
	
	param2_type SMALLINT NOT NULL,
	param2_int BIGINT NOT NULL,
	param2_string TEXT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,

	PRIMARY KEY(guild_id, local_id)
)
`

func init() {
	common.RegisterDBSchemas("cplogs", DBSchema)
}

type rawLogEntry struct {
	GuildID int64 `db:"guild_id"`
	LocalID int64 `db:"local_id"`

	AuthorID       int64  `db:"author_id"`
	AuthorUsername string `db:"author_username"`

	Action string `db:"action"`

	Param1Type   uint8  `db:"param1_type"`
	Param1Int    int64  `db:"param1_int"`
	Param1String string `db:"param1_string"`

	Param2Type   uint8  `db:"param2_type"`
	Param2Int    int64  `db:"param2_int"`
	Param2String string `db:"param2_string"`

	CreatedAt time.Time `db:"created_at"`
}

func (r *rawLogEntry) toLogEntry() *LogEntry {
	format, ok := actionFormats[r.Action]
	if !ok {
		format = &ActionFormat{
			FormatString: r.Action,
			Key:          r.Action,
			Found:        false,
		}
		// panic("unknown action format: " + r.Action)
	}

	entry := &LogEntry{
		GuildID: r.GuildID,
		LocalID: r.LocalID,

		AuthorID:       r.AuthorID,
		AuthorUsername: r.AuthorUsername,
		Action: &LogAction{
			FormatStr: format.FormatString,
			Key:       r.Action,
		},

		CreatedAt: r.CreatedAt,
	}

	if format.Found {
		if r.Param1Type > 0 {
			entry.Action.Params = append(entry.Action.Params, readParam(r.Param1Type, r.Param1Int, r.Param1String))
			if r.Param2Type > 0 {
				entry.Action.Params = append(entry.Action.Params, readParam(r.Param2Type, r.Param2Int, r.Param2String))
			}
		}
	}

	return entry
}

func readParam(paramType uint8, paramInt int64, paramString string) *Param {
	var value interface{}
	switch ParamType(paramType) {
	case ParamTypeInt:
		value = paramInt
	case ParamTypeString:
		value = paramString
	}

	return &Param{
		Type:  ParamType(paramType),
		Value: value,
	}

}

type LogEntry struct {
	GuildID int64
	LocalID int64

	AuthorID       int64
	AuthorUsername string

	Action *LogAction

	CreatedAt time.Time
}

func (l *LogEntry) toRawLogEntry() *rawLogEntry {
	rawEntry := &rawLogEntry{
		GuildID: l.GuildID,
		LocalID: l.LocalID,

		AuthorID:       l.AuthorID,
		AuthorUsername: l.AuthorUsername,

		Action:    l.Action.Key,
		CreatedAt: l.CreatedAt,
	}

	if len(l.Action.Params) > 0 {
		rawEntry.Param1Type = uint8(l.Action.Params[0].Type)
		switch t := l.Action.Params[0].Value.(type) {
		case int64:
			rawEntry.Param1Int = t
		case string:
			rawEntry.Param1String = t
		}

		if len(l.Action.Params) > 1 {
			rawEntry.Param2Type = uint8(l.Action.Params[1].Type)
			switch t := l.Action.Params[1].Value.(type) {
			case int64:
				rawEntry.Param2Int = t
			case string:
				rawEntry.Param2String = t
			}
		}
	}

	return rawEntry
}

func NewEntry(guildID int64, authorID int64, authorUsername string, action string, params ...*Param) *LogEntry {
	entry := &LogEntry{
		CreatedAt:      time.Now(),
		GuildID:        guildID,
		AuthorID:       authorID,
		AuthorUsername: authorUsername,

		Action: &LogAction{
			Key:    action,
			Params: params,
		},
	}

	return entry
}

type LogAction struct {
	Key       string
	FormatStr string
	Params    []*Param
}

func (l *LogAction) String() string {
	f := l.FormatStr
	args := []interface{}{}
	for _, v := range l.Params {
		args = append(args, v.Value)
	}

	return fmt.Sprintf(f, args...)
}

type ParamType uint8

const (
	ParamTypeNone   ParamType = 0
	ParamTypeInt    ParamType = 1
	ParamTypeString ParamType = 2
)

type Param struct {
	Type  ParamType
	Value interface{}
}
