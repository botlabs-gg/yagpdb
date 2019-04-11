package main

import (
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
	"log/syslog"
)

func AddSyslogHooks() {
	logrus.Println("Adding syslog hook")

	hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_DAEMON, flagNodeID)

	if err == nil {
		logrus.AddHook(hook)
	} else {
		logrus.WithError(err).Println("failed initializing syslog hook")
	}
}
