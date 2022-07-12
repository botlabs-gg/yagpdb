package bot

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
)

// ReadyTracker tracks process shards and initial readies/resumes, aswell as sending out events
var ReadyTracker = &readyTracker{}

type readyTracker struct {
	receivedReadyOrResume []bool
	allProcessShards      []bool
	totalShardCount       int
	mu                    sync.RWMutex
}

func (r *readyTracker) initTotalShardCount(count int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.totalShardCount = count
	r.receivedReadyOrResume = make([]bool, count)
	r.allProcessShards = make([]bool, count)
}

func (r *readyTracker) shardsAdded(shardIDs ...int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, v := range shardIDs {
		r.allProcessShards[v] = true
	}
	go eventsystem.QueueEventNonDiscord(eventsystem.NewEventData(nil, eventsystem.EventYagShardsAdded, shardIDs))
}

func (r *readyTracker) handleReadyOrResume(evt *eventsystem.EventData) {
	s := evt.Session

	r.mu.Lock()
	defer r.mu.Unlock()

	alreadyReady := r.receivedReadyOrResume[s.ShardID]
	if alreadyReady {
		return
	}

	r.receivedReadyOrResume[s.ShardID] = true
	go eventsystem.QueueEventNonDiscord(eventsystem.NewEventData(evt.Session, eventsystem.EventYagShardReady, s.ShardID))
}

func (r *readyTracker) shardRemoved(shardID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.receivedReadyOrResume[shardID] = false
	r.allProcessShards[shardID] = false
	go eventsystem.QueueEventNonDiscord(eventsystem.NewEventData(nil, eventsystem.EventYagShardRemoved, shardID))
}

// IsShardOnProcess returns true if the provided shard is on this process
func (r *readyTracker) IsShardOnProcess(shardID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.totalShardCount <= shardID {
		return false
	}

	return r.allProcessShards[shardID]
}

// IsShardReady returns true if the provided shard is ready
func (r *readyTracker) IsShardReady(shardID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.totalShardCount <= shardID {
		return false
	}

	return r.receivedReadyOrResume[shardID]
}

func (r *readyTracker) GetProcessShards() []int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	shards := make([]int, 0, totalShardCount)
	for s, active := range r.allProcessShards {
		if active {
			shards = append(shards, s)
		}
	}

	return shards
}

// IsGuildOnProcess returns true if the provided guild is on this process
func (r *readyTracker) IsGuildOnProcess(guildID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.totalShardCount == 0 {
		return false
	}

	shardID := GuildShardID(int64(r.totalShardCount), guildID)
	return r.allProcessShards[shardID]
}

// IsGuildShardReady returns true if the shard for the specified guild is ready
// note: it does not make sure that the guild create has been received for this guild
// it may still be unavailable in the state, but the shard has received a ready or resum
func (r *readyTracker) IsGuildShardReady(guildID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.totalShardCount == 0 {
		return false
	}

	shardID := GuildShardID(int64(r.totalShardCount), guildID)
	return r.receivedReadyOrResume[shardID]
}
