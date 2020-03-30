package internalapi

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"goji.io"
	"goji.io/pat"
)

var _ common.PluginWithCommonRun = (*Plugin)(nil)

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

var (
	confBotrestListenAddr = config.RegisterOption("yagpdb.botrest.listen_address", "botrest listening address, it will use any available port and make which port used avialable using service discovery (see service.go)", "127.0.0.1")
	ConfListenPortRange   = config.RegisterOption("yagpdb.botrest.port_range", "botrest listen port range", "5100-5999")
	serverLogger          = common.GetFixedPrefixLogger("internalapi_server")
)

// InternalAPIPlugin represents a plugin that provides interactions with the internal apis
type InternalAPIPlugin interface {
	InitInternalAPIRoutes(mux *goji.Mux)
}

type Plugin struct {
	srv   *http.Server
	srvMU sync.Mutex
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "internalapi",
		SysName:  "internalapi",
		Category: common.PluginCategoryCore,
	}
}

func (p *Plugin) CommonRun() {

	muxer := goji.NewMux()

	// muxer.HandleFunc(pat.Get("/:guild/guild"), HandleGuild)
	// muxer.HandleFunc(pat.Get("/:guild/botmember"), HandleBotMember)
	// muxer.HandleFunc(pat.Get("/:guild/members"), HandleGetMembers)
	// muxer.HandleFunc(pat.Get("/:guild/membercolors"), HandleGetMemberColors)
	// muxer.HandleFunc(pat.Get("/:guild/onlinecount"), HandleGetOnlineCount)
	// muxer.HandleFunc(pat.Get("/:guild/channelperms/:channel"), HandleChannelPermissions)
	// muxer.HandleFunc(pat.Get("/node_status"), HandleNodeStatus)
	// muxer.HandleFunc(pat.Post("/shard/:shard/reconnect"), HandleReconnectShard)
	muxer.HandleFunc(pat.Get("/ping"), handlePing)
	muxer.HandleFunc(pat.Post("/shutdown"), handleShutdown)

	// Debug stuff
	muxer.HandleFunc(pat.Get("/debug/pprof/cmdline"), pprof.Cmdline)
	muxer.HandleFunc(pat.Get("/debug/pprof/profile"), pprof.Profile)
	muxer.HandleFunc(pat.Get("/debug/pprof/symbol"), pprof.Symbol)
	muxer.HandleFunc(pat.Get("/debug/pprof/trace"), pprof.Trace)
	muxer.HandleFunc(pat.Get("/debug/pprof/"), pprof.Index)
	muxer.HandleFunc(pat.Get("/debug/pprof/:profile"), pprof.Index)

	// http.HandleFunc("/debug/pprof/", Index)
	// http.HandleFunc("/debug/pprof/cmdline", Cmdline)
	// http.HandleFunc("/debug/pprof/profile", Profile)
	// http.HandleFunc("/debug/pprof/symbol", Symbol)
	// http.HandleFunc("/debug/pprof/trace", Trace)

	for _, p := range common.Plugins {
		if botRestPlugin, ok := p.(InternalAPIPlugin); ok {
			botRestPlugin.InitInternalAPIRoutes(muxer)
		}
	}

	p.run(muxer)
}

func (p *Plugin) run(muxer *goji.Mux) {
	p.srv = &http.Server{
		Handler: muxer,
	}

	// listen address excluding port
	listenAddr := confBotrestListenAddr.GetString()
	if listenAddr == "" {
		// default to safe loopback interface
		listenAddr = "127.0.0.1"
	}

	l, port, err := p.createListener(listenAddr)
	if err != nil {
		serverLogger.WithError(err).Panicf("failed starting internal http server on %s:%d", listenAddr, port)
		return
	}

	common.ServiceTracker.SetAPIAddress(fmt.Sprintf("%s:%d", listenAddr, port))

	go func() {

		err := p.srv.Serve(l)
		if err != nil {
			if err == http.ErrServerClosed {
				serverLogger.Info("server closed, shutting down...")
				return
			}

			serverLogger.WithError(err).Error("failed serving internal api")
			return
		}
	}()
}

func (p *Plugin) createListener(addr string) (net.Listener, int, error) {
	ports, err := parseRange(ConfListenPortRange.GetString())
	if err != nil {
		panic(err)
	}

	for {

		for _, port := range ports {
			listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
			if err != nil {
				serverLogger.Infof("por %d used, trying another one...", port)
				time.Sleep(time.Second)
				continue
			}

			port := listener.Addr().(*net.TCPAddr).Port
			serverLogger.Infof("internalapi using port %d", port)
			return listener, port, nil
		}
	}
}

func ServeJson(w http.ResponseWriter, r *http.Request, data interface{}) {
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		serverLogger.WithError(err).Error("Failed sending json")
	}
}

// Returns true if an error occured
func ServerError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	encodedErr, _ := json.Marshal(err.Error())

	w.WriteHeader(http.StatusInternalServerError)
	w.Write(encodedErr)
	return true
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	ServeJson(w, r, "pong")
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	go func() {
		time.Sleep(time.Second * 3)
		common.Shutdown()
	}()

	ServeJson(w, r, "shutting down in 3 seconds")
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
