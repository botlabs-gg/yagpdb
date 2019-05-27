package main

import (
	"log/syslog"

	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func AddSyslogHooks() {
	logrus.Println("Adding syslog hook")

	hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_DAEMON, flagNodeID)

	if err == nil {
		common.AddLogHook(hook)
	} else {
		logrus.WithError(err).Println("failed initializing syslog hook")
	}
}
