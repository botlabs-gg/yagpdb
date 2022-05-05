package admin

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator/rest"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/bot_admin_panel.html
var PageHTMLPanel string

//go:embed assets/bot_admin_config.html
var PageHTMLConfig string

// InitWeb implements web.Plugin
func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("admin/assets/bot_admin_panel.html", PageHTMLPanel)
	web.AddHTMLTemplate("admin/assets/bot_admin_config.html", PageHTMLConfig)

	mux := goji.SubMux()
	web.RootMux.Handle(pat.New("/admin/*"), mux)
	web.RootMux.Handle(pat.New("/admin"), mux)

	mux.Use(web.RequireSessionMiddleware)
	mux.Use(web.RequireBotOwnerMW)

	panelHandler := web.ControllerHandler(p.handleGetPanel, "bot_admin_panel")

	mux.Handle(pat.Get(""), panelHandler)
	mux.Handle(pat.Get("/"), panelHandler)

	// Debug routes
	mux.Handle(pat.Get("/host/:host/pid/:pid/goroutines"), p.ProxyGetInternalAPI("/debug/pprof/goroutine"))
	mux.Handle(pat.Get("/host/:host/pid/:pid/trace"), p.ProxyGetInternalAPI("/debug/pprof/trace"))
	mux.Handle(pat.Get("/host/:host/pid/:pid/profile"), p.ProxyGetInternalAPI("/debug/pprof/profile"))
	mux.Handle(pat.Get("/host/:host/pid/:pid/heap"), p.ProxyGetInternalAPI("/debug/pprof/heap"))
	mux.Handle(pat.Get("/host/:host/pid/:pid/allocs"), p.ProxyGetInternalAPI("/debug/pprof/allocs"))

	// Control routes
	mux.Handle(pat.Post("/host/:host/pid/:pid/shutdown"), web.ControllerPostHandler(p.handleShutdown, panelHandler, nil))

	// Orhcestrator controls
	mux.Handle(pat.Post("/host/:host/pid/:pid/updateversion"), web.ControllerPostHandler(p.handleUpgrade, panelHandler, nil))
	mux.Handle(pat.Post("/host/:host/pid/:pid/migratenodes"), web.ControllerPostHandler(p.handleMigrateNodes, panelHandler, nil))
	mux.Handle(pat.Get("/host/:host/pid/:pid/deployedversion"), http.HandlerFunc(p.handleLaunchNodeVersion))

	// Node routes
	mux.Handle(pat.Get("/host/:host/pid/:pid/shard_sessions"), p.ProxyGetInternalAPI("/shard_sessions"))
	mux.Handle(pat.Post("/host/:host/pid/:pid/shard/:shardid/reconnect"), http.HandlerFunc(p.handleReconnectShard))

	mux.Handle(pat.Post("/reconnect_all"), http.HandlerFunc(p.handleReconnectAll))

	getConfigHandler := web.ControllerHandler(p.handleGetConfig, "bot_admin_config")
	mux.Handle(pat.Get("/config"), getConfigHandler)
	mux.Handle(pat.Post("/config/edit/:key"), web.ControllerPostHandler(p.handleEditConfig, getConfigHandler, nil))
}

type Host struct {
	Name         string
	ServiceHosts []*common.ServiceHost
}

func (p *Plugin) handleGetPanel(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	servicehosts, err := common.ServicePoller.GetActiveServiceHosts()
	if err != nil {
		return tmpl, errors.WithStackIf(err)
	}

	sort.Slice(servicehosts, func(i, j int) bool {
		for _, v := range servicehosts[i].Services {
			if v.Type == common.ServiceTypeBot {
				return false
			} else if v.Type == common.ServiceTypeOrchestator {
				return true
			}
		}

		for _, v := range servicehosts[j].Services {
			if v.Type == common.ServiceTypeBot {
				return true
			}
		}

		return false
	})

	hosts := make(map[string]*Host)
	for _, v := range servicehosts {
		if h, ok := hosts[v.Host]; ok {
			h.ServiceHosts = append(h.ServiceHosts, v)
			continue
		}

		hosts[v.Host] = &Host{
			Name:         v.Host,
			ServiceHosts: []*common.ServiceHost{v},
		}
	}

	tmpl["Hosts"] = hosts

	return tmpl, nil
}

func (p *Plugin) ProxyGetInternalAPI(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		debug := r.URL.Query().Get("debug")
		debugStr := ""
		if debug != "" {
			debugStr = "?debug=" + debug
		}

		sh, err := findServicehost(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error querying service hosts: " + err.Error()))
			return
		}

		resp, err := http.Get("http://" + sh.InternalAPIAddress + path + debugStr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error querying internal api: " + err.Error()))
			return
		}

		io.Copy(w, resp.Body)
	})
}

func (p *Plugin) handleShutdown(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	sh, err := findServicehost(r)
	if err != nil {
		return tmpl, err
	}

	var resp string
	err = internalapi.PostWithAddress(sh.InternalAPIAddress, "shutdown", nil, &resp)
	if err != nil {
		return tmpl, err
	}

	tmpl = tmpl.AddAlerts(web.SucessAlert(resp))
	return tmpl, nil
}

func (p *Plugin) handleUpgrade(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	client, err := createOrhcestatorRESTClient(r)
	if err != nil {
		return tmpl, err
	}

	logger.Println("Upgrading version...")

	newVer, err := client.PullNewVersion()
	if err != nil {
		tmpl.AddAlerts(web.ErrorAlert(err.Error()))
		return tmpl, err
	}

	tmpl = tmpl.AddAlerts(web.SucessAlert("Upgraded to ", newVer))
	return tmpl, nil
}

func (p *Plugin) handleMigrateNodes(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	client, err := createOrhcestatorRESTClient(r)
	if err != nil {
		return tmpl, err
	}

	logger.Println("Upgrading version...")

	response, err := client.MigrateAllNodesToNewNodes()
	if err != nil {
		tmpl.AddAlerts(web.ErrorAlert(err.Error()))
		return tmpl, err
	}

	tmpl = tmpl.AddAlerts(web.SucessAlert(response))
	return tmpl, nil
}

func (p *Plugin) handleLaunchNodeVersion(w http.ResponseWriter, r *http.Request) {
	logger.Println("ahahha")

	client, err := createOrhcestatorRESTClient(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error querying service hosts: " + err.Error()))
		return
	}

	ver, err := client.GetDeployedVersion()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error getting deployed version: " + err.Error()))
		return
	}

	w.Write([]byte(ver))
}

func createOrhcestatorRESTClient(r *http.Request) (*rest.Client, error) {
	sh, err := findServicehost(r)
	if err != nil {
		return nil, err
	}

	for _, v := range sh.Services {
		if v.Type == common.ServiceTypeOrchestator {
			return rest.NewClient("http://" + sh.InternalAPIAddress), nil
		}
	}

	return nil, common.ErrNotFound
}

func findServicehost(r *http.Request) (*common.ServiceHost, error) {
	host := pat.Param(r, "host")
	pid := pat.Param(r, "pid")

	serviceHosts, err := common.ServicePoller.GetActiveServiceHosts()
	if err != nil {
		return nil, err
	}

	for _, v := range serviceHosts {
		if v.Host == host && pid == strconv.Itoa(v.PID) {
			return v, nil
		}
	}

	return nil, common.ErrNotFound
}

func (p *Plugin) handleGetConfig(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	tmpl["ConfigOptions"] = config.Singleton.Options

	return tmpl, nil
}

func (p *Plugin) handleEditConfig(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())

	key := pat.Param(r, "key")
	value := r.FormValue("value")

	opt, ok := config.Singleton.Options[key]
	if !ok {
		return tmpl.AddAlerts(web.ErrorAlert("Unknown option")), nil
	}

	if opt.DefaultValue != nil {
		switch opt.DefaultValue.(type) {
		case int, int64:
			_, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return tmpl.AddAlerts(web.ErrorAlert("Value is not an integer")), nil
			}
		case bool:
			possibleChoices := []string{
				"true",
				"false",
				"yes",
				"no",
				"on",
				"off",
				"enabled",
				"disabled",
				"1",
				"0",
			}

			lower := strings.ToLower(value)
			if !common.ContainsStringSlice(possibleChoices, lower) {
				return tmpl.AddAlerts(web.ErrorAlert("Value is not a boolean")), nil
			}
		}
	}

	cs := config.RedisConfigStore{Pool: common.RedisPool}
	err := cs.SaveValue(key, value)
	if err != nil {
		return tmpl, err
	}

	return tmpl, nil
}

func (p *Plugin) handleGetShardSessions(w http.ResponseWriter, r *http.Request) {
	client, err := createOrhcestatorRESTClient(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error querying service hosts: " + err.Error()))
		return
	}

	ver, err := client.GetDeployedVersion()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error getting deployed version: " + err.Error()))
		return
	}

	w.Write([]byte(ver))
}

func (p *Plugin) handleReconnectShard(w http.ResponseWriter, r *http.Request) {

	forceReidentify := r.URL.Query().Get("identify") == "1"
	shardID := pat.Param(r, "shardid")

	sh, err := findServicehost(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error querying service hosts: " + err.Error()))
		return
	}

	queryParams := ""
	if forceReidentify {
		queryParams = "?reidentify=1"
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/shard/%s/reconnect%s", sh.InternalAPIAddress, shardID, queryParams), "", nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error querying internal api: " + err.Error()))
		return
	}

	io.Copy(w, resp.Body)
}

func (p *Plugin) handleReconnectAll(w http.ResponseWriter, r *http.Request) {

	totalShards, err := common.ServicePoller.GetShardCount()
	if err != nil {
		logger.WithError(err).Error("failed getting total shard count")
		w.Write([]byte("failed"))
		return
	}

	for i := 0; i < totalShards; i++ {
		err = botrest.SendReconnectShard(i, true)
		if err != nil {
			fmt.Fprintf(w, "Failed restarting %d\n", i)
			logger.WithError(err).Error("failed restarting shard")
		} else {
			fmt.Fprintf(w, "Restarted %d", i)
			logger.Infof("restarted shard %d", i)
		}

		time.Sleep(time.Second * 5)
	}
}
