package cacheset

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Manager struct {
	slots   []*Slot
	TTL     time.Duration
	started bool
}

func NewManager(ttl time.Duration) *Manager {
	return &Manager{
		TTL: ttl,
	}
}

func (m *Manager) RunGCLoop() {
	m.started = true

	if len(m.slots) < 1 {
		// No slots?
		return
	}

	t := time.NewTicker(time.Minute)
	i := 0
	for {
		<-t.C

		slot := m.slots[i]
		slot.gc(time.Now())

		i++
		if i >= len(m.slots) {
			i = 0
		}
	}
}

func (m *Manager) EvictSlotEntry(slot string, key interface{}) {
	for _, v := range m.slots {
		if v.name == slot {
			v.Delete(key)
		}
	}
}

func (m *Manager) FindSlot(slot string) *Slot {
	for _, v := range m.slots {
		if v.name == slot {
			return v
		}
	}

	return nil
}

type FetcherFunc = func(key interface{}) (interface{}, error)

// RegisterSlot register a new cached "thing"
// this is only safe to called during init() and friends
func (m *Manager) RegisterSlot(name string, fetcher FetcherFunc, keyType interface{}) *Slot {
	if m.started {
		panic("tried adding slots after manager had started")
	}

	for _, v := range m.slots {
		if v.name == name {
			panic(fmt.Sprintf("Key %s already used!", name))
		}
	}

	slot := &Slot{
		manager:  m,
		name:     name,
		fetcher:  fetcher,
		values:   make(map[interface{}]*cachedEntry),
		fetching: make(map[interface{}]*sync.Cond),
		keyType:  reflect.TypeOf(keyType),
	}

	m.slots = append(m.slots, slot)

	return slot
}

type Slot struct {
	manager *Manager

	name    string
	fetcher FetcherFunc

	valuesmu sync.RWMutex
	values   map[interface{}]*cachedEntry
	fetching map[interface{}]*sync.Cond

	keyType reflect.Type
}

func (s *Slot) Name() string {
	return s.name
}

type cachedEntry struct {
	value         interface{}
	expiresAt     time.Time
	accessCounter *int64
}

func (s *Slot) Get(key interface{}) (interface{}, error) {
	return s.GetCustomFetch(key, s.fetcher)
}

func (s *Slot) GetCustomFetch(key interface{}, fetcher FetcherFunc) (interface{}, error) {
	// fast path
	if v := s.getNoFetch(key); v != nil {
		metricsCacheHits.Add(1)
		return v, nil
	}

	metricsCacheMisses.Add(1)

	// item was not in cache, we need to fetch it
	s.valuesmu.Lock()
	for {
		if v := s.getValueLocked(key); v != nil {
			// we have the value now!
			s.valuesmu.Unlock()
			return v, nil
		}

		if c, ok := s.fetching[key]; ok {
			// someone else is fetching this item, wait
			c.Wait()
		} else {
			// we are not currently fetching this item, perform a a fetch
			c = sync.NewCond(&s.valuesmu)
			s.fetching[key] = c

			// unlock while were fetching to allow work on other guild's values
			s.valuesmu.Unlock()

			v, err := s.fetch(fetcher, key)

			s.valuesmu.Lock()
			if err == nil {
				// we successfully retrieved a value, put it in a cached
				s.values[key] = &cachedEntry{
					value:         v,
					expiresAt:     time.Now().Add(s.manager.TTL),
					accessCounter: new(int64),
				}
			}

			// no longer fetching this item
			delete(s.fetching, key)

			// wake up all waiters
			c.Broadcast()
			s.valuesmu.Unlock()

			return v, err
		}
	}
}

func (s *Slot) getNoFetch(key interface{}) interface{} {
	s.valuesmu.RLock()
	defer s.valuesmu.RUnlock()

	return s.getValueLocked(key)
}

func (s *Slot) getValueLocked(key interface{}) interface{} {
	if v, ok := s.values[key]; ok {
		return v.value
	}

	return nil
}

func (s *Slot) Delete(key interface{}) {
	s.valuesmu.Lock()
	defer s.valuesmu.Unlock()

	delete(s.values, key)
}

func (s *Slot) DeleteFunc(f func(key interface{}, value interface{}) bool) int {
	s.valuesmu.Lock()
	defer s.valuesmu.Unlock()

	n := 0
	for k, v := range s.values {
		if f(k, v.value) {
			delete(s.values, k)
			n++
		}
	}

	return n
}

func (s *Slot) fetch(fetcher FetcherFunc, key interface{}) (interface{}, error) {
	return fetcher(key)
}

func (s *Slot) gc(t time.Time) {
	s.valuesmu.Lock()
	defer s.valuesmu.Unlock()

	for k, v := range s.values {
		if v.expired(t) {
			delete(s.values, k)
		}
	}
}

func (s *Slot) NewKey() interface{} {
	return reflect.New(s.keyType).Interface()
}

func (e *cachedEntry) expired(t time.Time) bool {
	return t.After(e.expiresAt)
}

var (
	metricsCacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yagpdb_cacheset_cache_hits_total",
		Help: "Cache hits in the satte cache",
	})

	metricsCacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yagpdb_cacheset_cache_misses_total",
		Help: "Cache misses in the sate cache",
	})
)
