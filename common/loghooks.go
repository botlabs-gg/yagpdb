package common

import (
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

func init() {
	discordgo.Logger = discordLogger
	discordgo.GatewayLogger = DiscordGatewayLogger
}

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

	fu := runtime.FuncForPC(pc[0] - 1)
	name := fu.Name()
	file, line := fu.FileLine(pc[0] - 1)

	stack := filepath.Base(name) + ":" + filepath.Base(file) + ":" + strconv.Itoa(line)

	logLine := string(b)
	if strings.HasSuffix(logLine, "\n") {
		logLine = strings.TrimSuffix(logLine, "\n")
	}

	l := logrus.WithField("stck", stack)
	if strings.Contains(strings.ToLower(logLine), "error") {
		l.Error(logLine)
	} else {
		l.Info(logLine)
	}

	return
}

func discordLogger(msgL, caller int, format string, a ...interface{}) {
	pc := make([]uintptr, 3)
	runtime.Callers(caller+1, pc)
	fu := runtime.FuncForPC(pc[0] - 1)
	name := fu.Name()
	file, line := fu.FileLine(pc[0] - 1)

	stack := filepath.Base(name) + ":" + filepath.Base(file) + ":" + strconv.Itoa(line)

	f := logrus.WithField("stck", stack)

	switch msgL {
	case 0:
		f.Errorf("[DG] "+format, a...)
	case 1:
		f.Warnf("[DG] "+format, a...)
	default:
		f.Infof("[DG] "+format, a...)
	}
}

func DiscordGatewayLogger(shardID int, connectionID int, msgL int, msgf string, args ...interface{}) {
	pc := make([]uintptr, 3)
	runtime.Callers(3, pc)
	fu := runtime.FuncForPC(pc[0] - 1)
	name := fu.Name()
	file, line := fu.FileLine(pc[0] - 1)

	stack := filepath.Base(name) + ":" + filepath.Base(file) + ":" + strconv.Itoa(line)

	f := logrus.WithField("stck", stack).WithField("shard", shardID).WithField("connid", connectionID)

	switch msgL {
	case 0:
		f.Errorf("[GATEWAY] "+msgf, args...)
	case 1:
		f.Warnf("[GATEWAY] "+msgf, args...)
	default:
		f.Infof("[GATEWAY] "+msgf, args...)
	}
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

var (
	metricsNumRequestsPath = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yagpdb_discord_http_requests_path_total",
		Help: "Number of http requests to the discord API",
	}, []string{"path"})

	metricsNumRequestsResponseCode = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yagpdb_discord_http_requests_response_code_total",
		Help: "Number of http requests to the discord API",
	}, []string{"response_code"})

	metricsHTTPLatency = promauto.NewSummary(prometheus.SummaryOpts{
		Name:       "yagpdb_discord_http_latency_seconds",
		Help:       "Latency do the discord API",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
)

func (t *LoggingTransport) RoundTrip(request *http.Request) (*http.Response, error) {

	bucketI := request.Context().Value(discordgo.CtxKeyRatelimitBucket)
	var rlBucket *discordgo.Bucket
	if bucketI != nil {
		rlBucket = bucketI.(*discordgo.Bucket)
	}

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

	since := time.Since(started).Seconds()
	go func() {
		path := "unknown"
		if rlBucket != nil {
			// path = rlBucket.Key
			path = strings.Replace(rlBucket.Key, "https://discordapp.com/api/v", "", 1)
		}

		path = numberRemover.Replace(path)

		metricsHTTPLatency.Observe(since)
		// metricsNumRequests.With(prometheus.Labels{"path": path})
		metricsNumRequestsResponseCode.With(prometheus.Labels{"response_code": strconv.Itoa(code)}).Inc()

		// if Statsd != nil {
		// 	Statsd.Incr("discord.num_requests", []string{"method:" + request.Method, "resp_code:" + strconv.Itoa(code), "path:" + request.Method + "-" + path}, 1)
		// 	Statsd.Gauge("discord.http_latency", since, nil, 1)
		// 	if code == 429 {
		// 		Statsd.Incr("discord.requests.429", []string{"method:" + request.Method, "path:" + request.Method + "-" + path}, 1)
		// 	}
		// }

		if since > 5000 {
			logrus.WithField("path", request.URL.Path).WithField("ms", since).WithField("method", request.Method).Warn("Request took longer than 5 seconds to complete!")
		}

		// Statsd.Incr("discord.response.code."+strconv.Itoa(floored), nil, 1)
		// Statsd.Incr("discord.request.method."+request.Method, nil, 1)
	}()

	return resp, err
}

func AddLogHook(hook logrus.Hook) {
	logrus.AddHook(hook)
}

func SetLoggingLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

func SetLogFormatter(formatter logrus.Formatter) {
	logrus.SetFormatter(formatter)
}

func GetPluginLogger(plugin Plugin) *logrus.Entry {
	info := plugin.PluginInfo()
	return logrus.WithField("p", info.SysName)
}

func GetFixedPrefixLogger(prefix string) *logrus.Entry {
	return logrus.WithField("p", prefix)
}
