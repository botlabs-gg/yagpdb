package util

import (
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
)

type Command interface {
  EventHandler() ([]eventsystem.Event, eventsystem.Handler)
  YAGCommand() *commands.YAGCommand
}

type BaseCmd struct {}

func (c BaseCmd) EventHandler() ([]eventsystem.Event, eventsystem.Handler) {
  return nil, nil
}

