package commands

import (
	"github.com/jonas747/yagpdb/common"
	"time"
)

type LoggedExecutedCommand struct {
	common.SmallModel

	UserID    string
	ChannelID string
	GuildID   string

	// Name of command that was triggered
	Command string
	// Raw command with arguments passed
	RawCommand string
	// If command returned any error this will be no-empty
	Error string

	TimeStamp    time.Time
	ResponseTime int64
}

func (l LoggedExecutedCommand) TableName() string {
	return "executed_commands"
}
