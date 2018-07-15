// +build !linux

package main

import (
	"github.com/sirupsen/logrus"
)

func AddSyslogHooks() {
	logrus.Warn("Not on linux, cannot add syslog hooks")
}
