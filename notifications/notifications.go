package notifications

import (
	"goji.io"
	"goji.io/pat"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Notifications"
}

func (p *Plugin) InitBot() {}

func (p *Plugin) InitWeb(mainMuxer, cpMuxer *goji.Mux) {
	cpMuxer.HandleFuncC(pat.Get("/cp/:server/notifications"), h)
	cpMuxer.HandleFuncC(pat.Get("/cp/:server/notifications/"), h)
}
