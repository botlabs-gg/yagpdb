package backgroundworkers

import (
	"context"
	"net/http"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"goji.io"
)

var HTTPAddr = config.RegisterOption("yagpdb.bgworker.http_server_addr", "Backgroundn worker http server address", "localhost:5004")
var RESTServerMuxer *goji.Mux

var restServer *http.Server

var logger = common.GetFixedPrefixLogger("bgworkers")

type BackgroundWorkerPlugin interface {
	RunBackgroundWorker()
	StopBackgroundWorker(wg *sync.WaitGroup)
}

func RunWorkers() {
	common.ServiceTracker.RegisterService(common.ServiceTypeBGWorker, "Background worker", "", nil)

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
		Addr:    HTTPAddr.GetString(),
	}

	err := restServer.ListenAndServe()
	if err != nil {
		logger.WithError(err).Error("Failed starting http server")
	}
}
