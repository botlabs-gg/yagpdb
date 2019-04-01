package safebrowsing

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/sirupsen/logrus"
	"goji.io/pat"
	"net/http"
	"sync"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

type Plugin struct {
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "SafeBrowsing",
		SysName:  "safe_browsing",
		Category: common.PluginCategoryModeration,
	}
}

func (p *Plugin) RunBackgroundWorker() {
	if SafeBrowser == nil {
		err := runDatabase()
		if err != nil {
			logrus.WithError(err).Error("[safebrowsing] failed starting database")
			return
		}
	}

	backgroundworkers.RESTServerMuxer.Handle(pat.Post("/safebroswing/checkmessage"), http.HandlerFunc(handleCheckMessage))
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {

	if SafeBrowser != nil {
		SafeBrowser.Close()
	}

	wg.Done()
}
