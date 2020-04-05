package common

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/mediocregopher/radix/v3"
)

const ServicesRedisKey = "yag_services"

type serviceTracker struct {
	host *ServiceHost

	lastUpdate []byte

	mu sync.Mutex
}

// ServiceType represents the type of the component
type ServiceType string

const (
	ServiceTypeBot         ServiceType = "bot"
	ServiceTypeFrontend    ServiceType = "frontend"
	ServiceTypeBGWorker    ServiceType = "bgworker"
	ServiceTypeFeed        ServiceType = "feed"
	ServiceTypeOrchestator ServiceType = "orchestrator"
)

// Service represents a service or component of yagpdb
type Service struct {
	Type    ServiceType `json:"type"`
	Name    string      `json:"name"`
	Details string      `json:"details"`

	botDetailsF func() (*BotServiceDetails, error)
	BotDetails  *BotServiceDetails `json:"bot_details"`
}

// BotServiceDetails is bot service specific details
type BotServiceDetails struct {
	RunningShards    []int  `json:"running_shards"`
	TotalShards      int    `json:"total_shards"`
	NodeID           string `json:"node_id"`
	OrchestratorMode bool   `json:"orchestrator_mode"`
}

// ServiceHost represents a process that holds oen or more bot components
type ServiceHost struct {
	InternalAPIAddress string `json:"internal_api_address"`
	Host               string `json:"host"`
	PID                int    `json:"pid"`
	Version            string `json:"version"`

	Services []*Service `json:"services"`
}

// ServiceTracker keeps track of the various components of yagpdb in a central location for ease of access
var ServiceTracker = newServiceTracker()

func newServiceTracker() *serviceTracker {
	hostname, _ := os.Hostname()

	st := &serviceTracker{
		host: &ServiceHost{
			Host:    hostname,
			PID:     os.Getpid(),
			Version: VERSION,
		},
	}

	go st.run()

	return st
}

func (s *serviceTracker) RegisterService(t ServiceType, name string, details string, extrasF func() (*BotServiceDetails, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.host.Services = append(s.host.Services, &Service{
		Type:        t,
		Name:        name,
		Details:     details,
		botDetailsF: extrasF,
	})
}

func (s *serviceTracker) SetAPIAddress(apiAddress string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.host.InternalAPIAddress = apiAddress
}

func (s *serviceTracker) run() {
	t := time.NewTicker(time.Second * 5)
	for {
		<-t.C
		s.update()
	}
}

func (s *serviceTracker) update() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range s.host.Services {
		if v.botDetailsF == nil {
			continue
		}

		botDetails, err := v.botDetailsF()
		if err != nil {
			logger.WithError(err).Error("failed retrieving extra service details")
			v.BotDetails = &BotServiceDetails{}
			continue
		}

		v.BotDetails = botDetails
	}

	serialized, err := json.Marshal(s.host)
	if err != nil {
		logger.WithError(err).Error("failed marshaling service host")
		return
	}

	if !bytes.Equal(serialized, s.lastUpdate) {
		err = RedisPool.Do(radix.FlatCmd(nil, "ZREM", ServicesRedisKey, s.lastUpdate))
		if err != nil {
			logger.WithError(err).Error("failed removing service host")
			return
		}
	}

	err = RedisPool.Do(radix.FlatCmd(nil, "ZADD", ServicesRedisKey, time.Now().Unix(), serialized))
	if err != nil {
		logger.WithError(err).Error("failed updating service host")
		return
	}

	s.lastUpdate = serialized

	err = RedisPool.Do(radix.FlatCmd(nil, "ZREMRANGEBYSCORE", ServicesRedisKey, 0, time.Now().Unix()-30))
	if err != nil {
		logger.WithError(err).Error("feailed clearing old service hosts")
		return
	}
}

type servicePoller struct {
	cachedServiceHosts []*ServiceHost
	lastPoll           time.Time
	mu                 sync.Mutex
}

var ServicePoller = &servicePoller{}

func (sp *servicePoller) getActiveServiceHosts() ([]*ServiceHost, error) {
	if time.Since(sp.lastPoll) < time.Second*5 {
		return sp.cachedServiceHosts, nil
	}

	var hosts []string

	err := RedisPool.Do(radix.FlatCmd(&hosts, "ZRANGE", ServicesRedisKey, 0, -1))
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	result := make([]*ServiceHost, 0, len(hosts))
	for _, v := range hosts {
		var parsed *ServiceHost
		err = json.Unmarshal([]byte(v), &parsed)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		result = append(result, parsed)
	}

	sp.cachedServiceHosts = result
	sp.lastPoll = time.Now()

	return result, nil
}

// GetActiveServiceHosts returns all of the running service providers of the bot (processes)
func (sp *servicePoller) GetActiveServiceHosts() ([]*ServiceHost, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	return sp.getActiveServiceHosts()
}

// GetShardAddress returns the internal api address of the specified shard
func (sp *servicePoller) GetShardAddress(shardID int) (string, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	hosts, err := sp.getActiveServiceHosts()
	if err != nil {
		return "", err
	}

	for _, h := range hosts {
		for _, v := range h.Services {
			if v.Type == ServiceTypeBot && ContainsIntSlice(v.BotDetails.RunningShards, shardID) {
				return h.InternalAPIAddress, nil
			}
		}
	}

	return "", ErrNotFound
}

// GetGuildAddress returns the internal api addrress of the specified shard
// This is preferred over GetShardAddress as it also handles cases of different total shard couns (mid upscaling for example)
func (sp *servicePoller) GetGuildAddress(guildID int64) (string, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	hosts, err := sp.getActiveServiceHosts()
	if err != nil {
		return "", err
	}

	for _, h := range hosts {
		for _, v := range h.Services {
			if v.Type != ServiceTypeBot {
				continue
			}

			shardID := int((guildID >> 22) % int64(v.BotDetails.TotalShards))

			if ContainsIntSlice(v.BotDetails.RunningShards, shardID) {
				return h.InternalAPIAddress, nil
			}
		}
	}

	return "", ErrNotFound
}

// GetNodeAddress returns the internal api address of the specified nodeID
func (sp *servicePoller) GetNodeAddress(nodeID string) (string, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	hosts, err := sp.getActiveServiceHosts()
	if err != nil {
		return "", err
	}

	for _, h := range hosts {
		for _, v := range h.Services {
			if v.BotDetails != nil && v.BotDetails.NodeID == nodeID {
				return h.InternalAPIAddress, nil
			}
		}
	}

	return "", ErrNotFound
}

// GetShardCount returns the total shard count from the first node with a bot, or ErrNotFound otherwise
func (sp *servicePoller) GetShardCount() (int, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	hosts, err := sp.getActiveServiceHosts()
	if err != nil {
		return 0, err
	}

	for _, h := range hosts {
		for _, v := range h.Services {
			if v.Type == ServiceTypeBot {
				return v.BotDetails.TotalShards, nil
			}
		}
	}

	return 0, ErrNotFound
}
