package shardmemberfetcher

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate/inmemorytracker"
	"github.com/karlseguin/ccache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ReadyTracker interface {
	IsShardReady(shardID int) bool
}

var metricsRequests = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_memberfetcher_requests_total",
	Help: "The total number members added to queue",
})

var metricsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "yagpdb_memberfetcher_processed_total",
	Help: "The total number of processed queue items",
}, []string{"type"})

var metricsFailed = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "yagpdb_memberfetcher_failed_total",
	Help: "The total number of failed queue items",
}, []string{"type"})

var metricsGatewayChunkFailed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_memberfetcher_gateway_chunk_fail_total",
	Help: "The number of failed",
})

type GatewayRequestFunc func(guildID int64, userIDs []int64, nonce string) error

type Manager struct {
	state       dstate.StateTracker
	totalShards int64

	fetchers      []*shardMemberFetcher
	fetchersMu    sync.RWMutex
	gwRequestFunc GatewayRequestFunc

	failedUsersCache *ccache.Cache
	rTracker         ReadyTracker
}

func NewManager(totalShards int64, state dstate.StateTracker, f GatewayRequestFunc, rt ReadyTracker) *Manager {
	return &Manager{
		totalShards:      totalShards,
		state:            state,
		gwRequestFunc:    f,
		failedUsersCache: ccache.New(ccache.Configure()),
		rTracker:         rt,
	}
}

func (m *Manager) GetMember(guildID, userID int64) (*dstate.MemberState, error) {
	return m.getMember(guildID, userID, false)
}

func (m *Manager) GetMembers(guildID int64, userIDs ...int64) ([]*dstate.MemberState, error) {
	result := make([]*dstate.MemberState, 0, len(userIDs))

	resultChan := make(chan *MemberFetchResult)
	requests := make([]*MemberFetchRequest, 0, len(userIDs))

	for _, v := range userIDs {

		// check state first
		ms := m.state.GetMember(guildID, v)
		if ms != nil && ms.Member != nil {
			result = append(result, ms)
			continue
		}

		// otherwise create a request
		requests = append(requests, &MemberFetchRequest{
			resp:   resultChan,
			Member: v,
			Guild:  guildID,
		})
	}

	fetcher := m.findCreateFetcher(guildID)
	fetcher.reqChan <- requests

	for range requests {
		r := <-resultChan
		if r.Member != nil {
			result = append(result, r.Member)
		}
	}

	return result, nil
}

func (m *Manager) getMember(guildID, userID int64, joinedAt bool) (*dstate.MemberState, error) {
	// check from state first

	ms := m.state.GetMember(guildID, userID)
	if ms != nil && ms.Member != nil && (!joinedAt || ms.Member.JoinedAt != "") {
		return ms, nil
	}

	// make the request
	resultChan := make(chan *MemberFetchResult)

	req := []*MemberFetchRequest{
		{
			resp:   resultChan,
			Member: userID,
			Guild:  guildID,
		},
	}

	fetcher := m.findCreateFetcher(guildID)
	fetcher.reqChan <- req

	result := <-resultChan
	return result.Member, result.Err
}

func (m *Manager) findCreateFetcher(guildID int64) *shardMemberFetcher {
	shardID := (guildID >> 22) % m.totalShards

	// fast path
	m.fetchersMu.RLock()
	for _, v := range m.fetchers {
		if v.shardID == shardID {
			m.fetchersMu.RUnlock()
			return v
		}
	}
	m.fetchersMu.RUnlock()

	// no result, slow path
	m.fetchersMu.Lock()
	defer m.fetchersMu.Unlock()

	// check again as it could have changed inbetween upgrading the lock
	for _, v := range m.fetchers {
		if v.shardID == shardID {
			return v
		}
	}

	// still no result, make a new fetcher
	fetcher := &shardMemberFetcher{
		state:         m.state,
		gwRequestFunc: m.gwRequestFunc,
		reqChan:       make(chan []*MemberFetchRequest),
		shardID:       shardID,
		failedCache:   m.failedUsersCache,
		sortedQueue:   make(map[int64][]*MemberFetchRequest),

		fetchingSingle:  make(map[guildMemberIDPair]bool),
		fetchingGWState: &FetchingGWState{Finished: true},

		finishedSingle:  make(chan *MemberFetchResult),
		finishedGateway: make(chan *discordgo.GuildMembersChunk),
		rTracker:        m.rTracker,
	}

	go fetcher.run()

	m.fetchers = append(m.fetchers, fetcher)

	return fetcher
}

func (m *Manager) HandleGuildmembersChunk(evt *eventsystem.EventData) {
	shard := evt.Session.ShardID
	m.fetchersMu.RLock()

	for _, v := range m.fetchers {
		if v.shardID == int64(shard) {
			v.finishedGateway <- evt.GuildMembersChunk()
		}
	}

	m.fetchersMu.RUnlock()
}

type shardMemberFetcher struct {
	state dstate.StateTracker

	reqChan chan []*MemberFetchRequest

	shardID int64

	fetchingGWState *FetchingGWState

	// key is guildID
	sortedQueue map[int64][]*MemberFetchRequest

	fetchingSingle map[guildMemberIDPair]bool

	finishedSingle  chan *MemberFetchResult
	finishedGateway chan *discordgo.GuildMembersChunk

	failedCache *ccache.Cache

	gwRequestFunc GatewayRequestFunc

	rTracker ReadyTracker
}

func (s *shardMemberFetcher) run() {
	guildFetcherTicker := time.NewTicker(time.Second)

	for {
		select {
		case reqSlice := <-s.reqChan:
			s.addToQueue(reqSlice)
		case finG := <-s.finishedGateway:
			s.handleFinishedGateway(finG)
		case finS := <-s.finishedSingle:
			s.handleFinishedSingle(finS)
		case <-guildFetcherTicker.C:
			s.checkSendNextGatewayRequest()
		}
	}
}

func (s *shardMemberFetcher) addToQueue(elems []*MemberFetchRequest) {
	guildID := int64(0)
	for _, v := range elems {
		s.sortedQueue[v.Guild] = append(s.sortedQueue[v.Guild], v)
		guildID = v.Guild
	}

	metricsRequests.Add(float64(len(elems)))

	if len(elems) > 2 && s.fetchingGWState.Finished && time.Since(s.fetchingGWState.Started) > time.Second {
		s.checkSendNextGatewayRequest()
		s.checkSendNextAPICall(guildID)
	} else {
		s.checkSendNextAPICall(guildID)
	}
}

func (s *shardMemberFetcher) handleFinishedSingle(elem *MemberFetchResult) {
	guildID := elem.GuildID

	doneRequests := make([]*MemberFetchRequest, 0, 1)
	newQueue := make([]*MemberFetchRequest, 0, len(s.sortedQueue[guildID]))

	// find the finished requests and remake the queue without the finished requests
	for _, v := range s.sortedQueue[guildID] {
		if v.Member == elem.MemberID {
			doneRequests = append(doneRequests, v)
			continue
		}

		// not finished
		newQueue = append(newQueue, v)
	}

	s.sortedQueue[guildID] = newQueue

	// remove the fetching status
	delete(s.fetchingSingle, guildMemberIDPair{
		GuildID:  elem.GuildID,
		MemberID: elem.MemberID,
	})

	// send the results
	for _, req := range doneRequests {
		go s.sendResult(req, elem)
	}

	// cache errors
	if elem.Err != nil {
		failedCacheKey := discordgo.StrID(guildID) + ":" + discordgo.StrID(elem.MemberID)
		s.failedCache.Set(failedCacheKey, true, time.Minute)
	}

	s.checkSendNextAPICall(guildID)
}

func (s *shardMemberFetcher) handleFinishedGateway(chunk *discordgo.GuildMembersChunk) {
	guildID := chunk.GuildID

	doneRequests := make([]*MemberFetchRequest, 0, len(chunk.Members))
	newQueue := make([]*MemberFetchRequest, 0, len(s.sortedQueue[guildID]))

	// find the finished requests and remake the queue without the finished requests
	// remove the fetching status
	if s.fetchingGWState.Nonce == chunk.Nonce {

		// mark all the members we passed to the request done
	OUTER_MATCHED_ALL:
		for _, v := range s.sortedQueue[guildID] {
			for _, done := range s.fetchingGWState.Members {
				if v.Member == done {
					doneRequests = append(doneRequests, v)
					continue OUTER_MATCHED_ALL
				}
			}

			newQueue = append(newQueue, v)
		}

		// Also mark these requests done
		s.fetchingGWState.Members = nil
		s.fetchingGWState.Finished = true
	} else {
		// mismatched... but still check our queues in case we got lucky!
	OUTER:
		for _, v := range s.sortedQueue[guildID] {
			for _, done := range chunk.Members {
				if v.Member == done.User.ID {
					doneRequests = append(doneRequests, v)
					continue OUTER
				}
			}

			newQueue = append(newQueue, v)
		}
	}

	s.sortedQueue[guildID] = newQueue

	// send the results
OUTER_SEND:
	for _, req := range doneRequests {
		for _, v := range chunk.Members {
			if req.Member == v.User.ID {
				go s.sendGWResult(req, v)
				continue OUTER_SEND
			}
		}

		// not found, sendn il
		go s.sendGWResult(req, nil)
	}

	// add to state if we can
	cast, ok := s.state.(*inmemorytracker.InMemoryTracker)
	if !ok {
		return
	}

	for _, v := range chunk.Members {
		if ms := s.state.GetMember(guildID, v.User.ID); ms != nil && ms.Member != nil {
			continue // already in state
		}

		ms := dstate.MemberStateFromMember(v)
		ms.GuildID = guildID
		cast.SetMember(ms)
	}
}

func (s *shardMemberFetcher) sendResult(req *MemberFetchRequest, result *MemberFetchResult) {
	req.resp <- result
}

func (s *shardMemberFetcher) sendGWResult(req *MemberFetchRequest, member *discordgo.Member) {
	if member == nil {
		metricsFailed.With(prometheus.Labels{"type": "gateway"}).Inc()

		req.resp <- &MemberFetchResult{
			Member:   nil,
			Err:      errors.New("not found"),
			GuildID:  req.Guild,
			MemberID: req.Member,
		}
	} else {
		metricsProcessed.With(prometheus.Labels{"type": "gateway"}).Add(1)

		ms := dstate.MemberStateFromMember(member)
		ms.GuildID = req.Guild
		req.resp <- &MemberFetchResult{
			Err:      nil,
			Member:   ms,
			GuildID:  req.Guild,
			MemberID: req.Member,
		}
	}
}

func (s *shardMemberFetcher) checkSendNextAPICall(guildID int64) {
	for k := range s.fetchingSingle {
		if k.GuildID == guildID {
			// already fetching on this guild
			return
		}
	}

	// fetch
	if q, ok := s.sortedQueue[guildID]; ok && len(q) > 0 {
		var next *MemberFetchRequest
		if !s.fetchingGWState.Finished && s.fetchingGWState.GuildID == guildID {
		OUTER:
			for _, rq := range q {
				for _, mID := range s.fetchingGWState.Members {
					if mID == rq.Member {
						continue OUTER
					}
				}

				next = rq
				break
			}
		} else {
			next = q[0]
		}

		if next == nil {
			// nothing more to do, already fetching from gateway
			return
		}

		s.fetchingSingle[guildMemberIDPair{
			GuildID:  guildID,
			MemberID: next.Member,
		}] = true

		go s.fetchSingle(next)
	}
}

func (s *shardMemberFetcher) fetchSingle(req *MemberFetchRequest) {

	var err error
	var result *dstate.MemberState
	defer func() {
		// we always wanna mark it was finished even if it panics somehow as otherwise it would get completely stuck
		s.finishedSingle <- &MemberFetchResult{
			Err:      err,
			Member:   result,
			GuildID:  req.Guild,
			MemberID: req.Member,
		}
	}()

	result, err = s.fetchSingleInner(req)
}

func (s *shardMemberFetcher) fetchSingleInner(req *MemberFetchRequest) (*dstate.MemberState, error) {

	// check if its already in state first
	gs := s.state.GetGuild(req.Guild)
	if gs == nil {
		metricsFailed.With(prometheus.Labels{"type": "state"}).Inc()
		return nil, errors.New("guild not found in state")
	}

	// it was already existant in the state
	result := s.state.GetMember(req.Guild, req.Member)
	if result != nil && result.Member != nil {
		metricsProcessed.With(prometheus.Labels{"type": "state"}).Inc()
		return result, nil
	}

	// fetch from api
	var m *discordgo.Member
	m, err := common.BotSession.GuildMember(req.Guild, req.Member)
	if err != nil {
		metricsFailed.With(prometheus.Labels{"type": "http"}).Inc()
		return nil, err
	}

	metricsProcessed.With(prometheus.Labels{"type": "http"}).Inc()

	result = dstate.MemberStateFromMember(m)
	result.GuildID = req.Guild // yes this field is not set...

	// add to state if we can
	if cast, ok := s.state.(*inmemorytracker.InMemoryTracker); ok {
		if state_ms := s.state.GetMember(req.Guild, req.Member); state_ms == nil || state_ms.Member == nil {
			// make a copy because we handed out references above and the below function might mutate it
			cast.SetMember(result)
		}
	}

	return result, nil
}

func (s *shardMemberFetcher) checkSendNextGatewayRequest() {
	if !s.rTracker.IsShardReady(int(s.shardID)) {
		return
	}

	if !s.fetchingGWState.Finished {
		if time.Since(s.fetchingGWState.Started) > time.Minute {
			s.fetchingGWState.Finished = true
			s.fetchingGWState.Members = nil

			metricsGatewayChunkFailed.Inc()

			// trigger The number of failed case it never finished
			s.checkSendNextAPICall(s.fetchingGWState.GuildID)
		} else {
			return
		}
	}

	if time.Since(s.fetchingGWState.Started) < time.Second {
		// Keep it strictly 1 per second
		return
	}

	// find the biggest queue
	biggestQueueLen := 0
	biggestQueueGuild := int64(0)

	for g, q := range s.sortedQueue {
		l := len(q)
		if l > biggestQueueLen {
			biggestQueueGuild = g
			biggestQueueLen = l
		}
	}

	// no point in issuing a guild request if its below 2
	if biggestQueueLen < 1 {
		return
	}

	// fetch from this guild
	// generate a nonce
	ids := make([]int64, 0, 100)

OUTER:
	for _, v := range s.sortedQueue[biggestQueueGuild] {

		for _, existing := range ids {
			if v.Member == existing {
				// already added this member to the list
				continue OUTER
			}
		}

		if _, ok := s.fetchingSingle[guildMemberIDPair{
			GuildID:  biggestQueueGuild,
			MemberID: v.Member,
		}]; ok {
			// fetching this through normal API
			continue
		}

		ids = append(ids, v.Member)
		if len(ids) >= 100 {
			break
		}
	}

	if len(ids) < 1 {
		// in the cases of the queue being big but somehow they're all busy (don't think this is possible with the min requirement being len of 2 but whatever)
		return
	}

	nonce := fmt.Sprintf("shard_member_fetcher:%d", time.Now().UnixNano())
	s.fetchingGWState.GuildID = biggestQueueGuild
	s.fetchingGWState.Nonce = nonce
	s.fetchingGWState.Started = time.Now()
	s.fetchingGWState.Members = ids

	s.gwRequestFunc(biggestQueueGuild, ids, nonce)
}

type guildMemberIDPair struct {
	GuildID  int64
	MemberID int64
}

type FetchingGWState struct {
	Members  []int64
	Started  time.Time
	GuildID  int64
	Finished bool
	Nonce    string
}

type MemberFetchRequest struct {
	Member int64
	Guild  int64
	resp   chan *MemberFetchResult
}

type MemberFetchResult struct {
	Err      error
	Member   *dstate.MemberState
	GuildID  int64
	MemberID int64
}

type GWFetchResult struct {
	GuildID int64
	Members []*discordgo.Member
	Nonce   string
}
