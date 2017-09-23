package models

//go:generate kallax gen -e "kallax.go"

import (
	"gopkg.in/src-d/go-kallax.v1"
	"time"
)

type StatsPeriod struct {
	kallax.Model `table:"server_stats_periods" pk:"id,autoincr"`

	Started  time.Time
	Duration time.Duration

	ID        int64
	GuildID   int64
	UserID    int64
	ChannelID int64
	Count     int64
}
