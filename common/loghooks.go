package common

import (
	"github.com/Sirupsen/logrus"
	"path/filepath"
	"runtime"
	"strings"
)

type ContextHook struct{}

func (hook ContextHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook ContextHook) Fire(entry *logrus.Entry) error {
	// Skip if already provided
	if _, ok := entry.Data["line"]; ok {
		return nil
	}

	pc := make([]uintptr, 3)
	cnt := runtime.Callers(6, pc)

	for i := 0; i < cnt; i++ {
		fu := runtime.FuncForPC(pc[i] - 1)
		name := fu.Name()
		if !strings.Contains(name, "github.com/Sirupsen/logrus") {
			file, line := fu.FileLine(pc[i] - 1)
			entry.Data["file"] = filepath.Base(file)
			entry.Data["func"] = filepath.Base(name)
			entry.Data["line"] = line
			break
		}
	}
	return nil
}

type DGLogProxy struct{}

func (p *DGLogProxy) Write(b []byte) (n int, err error) {
	n = len(b)

	pc := make([]uintptr, 3)
	runtime.Callers(4, pc)

	data := make(logrus.Fields)

	fu := runtime.FuncForPC(pc[0] - 1)
	name := fu.Name()
	file, line := fu.FileLine(pc[0] - 1)
	data["file"] = filepath.Base(file)
	data["func"] = filepath.Base(name)
	data["line"] = line

	// for i := 0; i < cnt; i++ {
	// if !strings.Contains(name, "github.com/Sirupsen/logrus") {
	// }
	// }

	logrus.WithFields(data).Info(string(b))

	return
}
