package prom

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	ConfPromListenAddr      = config.RegisterOption("yagpdb.prom_listen_addr", "Prometheus listen address", "")
	ConfPromListenPortRange = config.RegisterOption("yagpdb.prom_listen_port_range", "Prometheus listen port range", "6001-6100")

	parsedPortRange []int
)

func RegisterPlugin() {
	var err error
	parsedPortRange, err = parseRange(ConfPromListenPortRange.GetString())
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	logrus.Infof("Using port range %v", parsedPortRange)
	if len(parsedPortRange) == 0 {
		logrus.Warn("No prom ports defined, not launching prom server")
		return
	}
	go startHTTPServer()
}

func startHTTPServer() {
	for {
		for _, p := range parsedPortRange {
			listenAddr := fmt.Sprintf("%s:%d", ConfPromListenAddr.GetString(), p)
			logrus.Infof("Attempting to start prom server on %s", listenAddr)
			err := http.ListenAndServe(listenAddr, promhttp.Handler())
			if err != nil {
				logrus.WithError(err).Warn("failed starting prom server, trying another port")
			}

			time.Sleep(time.Second)
		}
	}
}

func parseRange(in string) ([]int, error) {
	if in == "" {
		return nil, nil
	}

	if !strings.Contains(in, "-") {
		n, err := strconv.Atoi(in)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		return []int{n}, nil
	}

	split := strings.Split(in, "-")
	parsedStart, err := strconv.Atoi(split[0])
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	parsedEnd, err := strconv.Atoi(split[1])
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	result := make([]int, 0)
	for i := parsedStart; i <= parsedEnd; i++ {
		result = append(result, i)
	}

	return result, nil
}
