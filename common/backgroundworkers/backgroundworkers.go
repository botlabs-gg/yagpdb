package backgroundworkers

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"goji.io"
	"net/http"
	"os"
	"sync"
)

var HTTPAddr = loadHTTPAddr()
var RESTServerMuxer *goji.Mux

var restServer *http.Server

var logger = common.GetFixedPrefixLogger("bgworkers")

type BackgroundWorkerPlugin interface {
	RunBackgroundWorker()
	StopBackgroundWorker(wg *sync.WaitGroup)
}

func RunWorkers() {
	RESTServerMuxer = goji.NewMux()

	for _, p := range common.Plugins {
		if bwc, ok := p.(BackgroundWorkerPlugin); ok {
			logger.Info("Running background worker: ", p.PluginInfo().Name)
			go bwc.RunBackgroundWorker()
		}
	}

	go runWebserver()
}

func StopWorkers(wg *sync.WaitGroup) {
	logger.Info("Shutting down http server...")
	if restServer != nil {
		restServer.Shutdown(context.Background())
	}

	for _, p := range common.Plugins {
		if bwc, ok := p.(BackgroundWorkerPlugin); ok {
			logger.Info("Stopping background worker: ", p.PluginInfo().Name)
			wg.Add(1)
			go bwc.StopBackgroundWorker(wg)
		}
	}
}

func runWebserver() {
	logger.Info("Starting bgworker http server on ", HTTPAddr)

	restServer := &http.Server{
		Handler: RESTServerMuxer,
		Addr:    HTTPAddr,
	}

	err := restServer.ListenAndServe()
	if err != nil {
		logger.WithError(err).Error("Failed starting http server")
	}
}

func loadHTTPAddr() string {
	addr := os.Getenv("YAGPDB_BGWORKER_HTTP_SERVER_ADDR")
	if addr == "" {
		addr = "localhost:5004"
	}

	return addr
}
