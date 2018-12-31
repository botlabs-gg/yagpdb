package common

import (
	"github.com/sirupsen/logrus"
	"math"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ContextHook struct{}

func (hook ContextHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook ContextHook) Fire(entry *logrus.Entry) error {
	// Skip if already provided
	if _, ok := entry.Data["stck"]; ok {
		return nil
	}

	pc := make([]uintptr, 3)
	cnt := runtime.Callers(6, pc)

	for i := 0; i < cnt; i++ {
		fu := runtime.FuncForPC(pc[i] - 1)
		name := fu.Name()
		if !strings.Contains(name, "github.com/sirupsen/logrus") {
			file, line := fu.FileLine(pc[i] - 1)

			entry.Data["stck"] = filepath.Base(name) + ":" + filepath.Base(file) + ":" + strconv.Itoa(line)
			break
		}
	}
	return nil
}

type STDLogProxy struct{}

func (p *STDLogProxy) Write(b []byte) (n int, err error) {
	n = len(b)

	pc := make([]uintptr, 3)
	runtime.Callers(4, pc)

	data := make(logrus.Fields)

	fu := runtime.FuncForPC(pc[0] - 1)
	name := fu.Name()
	file, line := fu.FileLine(pc[0] - 1)
	data["stck"] = filepath.Base(name) + ":" + filepath.Base(file) + ":" + strconv.Itoa(line)

	logLine := string(b)
	if strings.HasSuffix(logLine, "\n") {
		logLine = strings.TrimSuffix(logLine, "\n")
	}

	logrus.WithFields(data).Info(logLine)

	return
}

type GORMLogger struct {
}

func (g *GORMLogger) Print(v ...interface{}) {
	logrus.WithField("stck", "...").Error(v...)
}

type LoggingTransport struct {
	Inner http.RoundTripper
}

var numberRemover = strings.NewReplacer(
	"0", "",
	"1", "",
	"2", "",
	"3", "",
	"4", "",
	"5", "",
	"6", "",
	"7", "",
	"8", "",
	"9", "")

func (t *LoggingTransport) RoundTrip(request *http.Request) (*http.Response, error) {

	inner := t.Inner
	if inner == nil {
		inner = http.DefaultTransport
	}

	started := time.Now()

	code := 0
	resp, err := inner.RoundTrip(request)
	if resp != nil {
		code = resp.StatusCode
	}

	floored := int(math.Floor(float64(code) / 100))

	since := strconv.Itoa(int(time.Since(started).Seconds() * 1000))
	go func() {
		path := numberRemover.Replace(request.URL.Path)

		Statsd.Incr("discord.num_requsts", []string{"method:" + request.Method, "resp_code:" + strconv.Itoa(floored), "path:" + request.Method + "-" + path, "lat:" + since}, 1)
		if code == 429 {
			Statsd.Incr("discord.requests.429", []string{"method:" + request.Method, "path:" + request.Method + "-" + path}, 1)
		}

		// Statsd.Incr("discord.response.code."+strconv.Itoa(floored), nil, 1)
		// Statsd.Incr("discord.request.method."+request.Method, nil, 1)
	}()

	return resp, err
}
