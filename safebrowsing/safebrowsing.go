package safebrowsing

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"sync"
)

const serverAddr = "localhost:5004"

var _ common.BackgroundWorkerPlugin = (*Plugin)(nil)

type Plugin struct {
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

func (p *Plugin) Name() string {
	return "safebrowsing"
}

func (p *Plugin) RunBackgroundWorker() {
	err := RunServer()
	if err != nil {
		logrus.WithError(err).Error("[safebrowsing] failed starting safebrowsing server")
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	if restServer != nil {
		restServer.Shutdown(context.Background())
	}

	if SafeBrowser != nil {
		SafeBrowser.Close()
	}

	wg.Done()
}
