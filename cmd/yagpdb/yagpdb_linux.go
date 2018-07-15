package main

import (
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
	"log/syslog"
)

func AddSyslogHooks() {
	logrus.Println("Adding syslog hook")

	hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")

	if err == nil {
		logrus.AddHook(hook)
	} else {
		logrus.Println("failed initializing syslog hook")
	}
}
