package sentryhook

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

type Hook struct{}

func (hook Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (hook Hook) Fire(entry *logrus.Entry) error {
	hub := sentry.CurrentHub().Clone()
	if hub == nil {
		return nil
	}

	hub.WithScope(func(s *sentry.Scope) {
		// Skip if already provided
		for k, v := range entry.Data {
			strV := fmt.Sprint(v)
			switch k {
			case "p":
				s.SetTag("plugin", strV)
			case "guild", "g", "guild_id":
				s.SetExtra("guild_id", strV)
			case "stck":
			default:
				s.SetExtra(k, strV)
			}
		}

		if err, ok := entry.Data["error"]; ok {
			s.SetExtra("message", entry.Message)
			hub.CaptureException(err.(error))
		} else {
			hub.CaptureMessage(entry.Message)
		}
	})

	return nil
}
