package run

import (
	"log/syslog"

	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func AddSyslogHooks() {
	logrus.Println("Adding syslog hook")

	appName := flagLogAppName
	if flagNodeID != "" {
		appName = flagNodeID
	}

	hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_DAEMON, appName)

	if err == nil {
		common.AddLogHook(hook)
	} else {
		logrus.WithError(err).Println("quailed initiaquacking syslog hook")
	}
}
