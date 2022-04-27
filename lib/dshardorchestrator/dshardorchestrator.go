package dshardorchestrator

import (
	"log"
)

type ShardMigrationMode int

const (
	ShardMigrationModeNone ShardMigrationMode = iota
	ShardMigrationModeTo
	ShardMigrationModeFrom
)

type LogLevel int

const (
	LogError LogLevel = iota
	LogWarning
	LogInfo
	LogDebug
)

type Logger interface {
	Log(level LogLevel, message string)
}

var StdLogInstance = &StdLogger{Level: LogInfo}

type StdLogger struct {
	Level  LogLevel
	Prefix string
}

func (stdl *StdLogger) Log(level LogLevel, message string) {
	if stdl.Level < level {
		return
	}

	strLevel := ""
	switch level {
	case LogError:
		strLevel = "ERRO"
	case LogWarning:
		strLevel = "WARN"
	case LogInfo:
		strLevel = "INFO"
	case LogDebug:
		strLevel = "DEBG"
	}

	log.Printf("[%s] %s%s", strLevel, stdl.Prefix, message)
}

func ContainsInt(slice []int, i int) bool {
	for _, v := range slice {
		if v == i {
			return true
		}
	}

	return false
}

// ShardInfo represents basic shard session info
type ShardInfo struct {
	ShardID   int
	SessionID string
	Sequence  int64
}
