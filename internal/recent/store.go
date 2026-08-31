package recent

import (
	"container/heap"
	"sort"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type entry struct {
	signal     model.Signal
	observedAt time.Time
	generation uint64
}

type expiryItem struct {
	id         string
	observedAt time.Time
	generation uint64
}

type expiryHeap []expiryItem

func (h expiryHeap) Len() int { return len(h) }
func (h expiryHeap) Less(i, j int) bool {
	if h[i].observedAt.Equal(h[j].observedAt) {
		if h[i].id == h[j].id {
			return h[i].generation < h[j].generation
		}
		return h[i].id < h[j].id
	}
	return h[i].observedAt.Before(h[j].observedAt)
}
func (h expiryHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *expiryHeap) Push(value any) { *h = append(*h, value.(expiryItem)) }
func (h *expiryHeap) Pop() any {
	old := *h
	last := len(old) - 1
	value := old[last]
	old[last] = expiryItem{}
	*h = old[:last]
	return value
}

// Store is a bounded, concurrency-safe window of recently observed signals.
// It is intentionally not the durable source of truth; SQLite owns durability.
// This buffer exists so the scoring scanner can cheaply cluster the current
// attention window without loading the full history database every run.
//
// Expiry/capacity maintenance uses a min-heap so a high-volume Jetstream write
// costs O(log n) rather than scanning the entire window for every event.
type Store struct {
	mu             sync.Mutex
	entries        map[string]entry
	expiry         expiryHeap
	maxEntries     int
	ttl            time.Duration
	nextGeneration uint64
}

func New(maxEntries int, ttl time.Duration) *Store {
	if maxEntries <= 0 {
		maxEntries = 50_000
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	store := &Store{
		entries:    make(map[string]entry, min(maxEntries, 4096)),
		expiry:     make(expiryHeap, 0, min(maxEntries, 4096)),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
	heap.Init(&store.expiry)
	return store
}

func (s *Store) Upsert(signal model.Signal, observedAt time.Time) {
	if signal.ID == "" {
		return
	}
	now := time.Now().UTC()
	if observedAt.IsZero() {
		observedAt = now
	}
	observedAt = observedAt.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextGeneration++
	item := entry{signal: signal, observedAt: observedAt, generation: s.nextGeneration}
	s.entries[signal.ID] = item
	heap.Push(&s.expiry, expiryItem{id: signal.ID, observedAt: observedAt, generation: item.generation})
	s.pruneLocked(now)
}

func (s *Store) UpsertMany(values []model.Signal, observedAt time.Time) {
	now := time.Now().UTC()
	if observedAt.IsZero() {
		observedAt = now
	}
	observedAt = observedAt.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, signal := range values {
		if signal.ID == "" {
			continue
		}
		s.nextGeneration++
		item := entry{signal: signal, observedAt: observedAt, generation: s.nextGeneration}
		s.entries[signal.ID] = item
		heap.Push(&s.expiry, expiryItem{id: signal.ID, observedAt: observedAt, generation: item.generation})
	}
	s.pruneLocked(now)
}

func (s *Store) Delete(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	delete(s.entries, id)
	s.pruneLocked(time.Now().UTC())
	s.mu.Unlock()
}

// Recent returns signals observed at or after since, ordered from oldest to
// newest observation. Callers receive values, not internal references.
func (s *Store) Recent(since time.Time) []model.Signal {
	now := time.Now().UTC()
	if since.IsZero() {
		since = now.Add(-s.ttl)
	}

	s.mu.Lock()
	s.pruneLocked(now)
	values := make([]entry, 0, len(s.entries))
	for _, item := range s.entries {
		if !item.observedAt.Before(since) {
			values = append(values, item)
		}
	}
	s.mu.Unlock()

	sort.SliceStable(values, func(i, j int) bool {
		if values[i].observedAt.Equal(values[j].observedAt) {
			return values[i].signal.ID < values[j].signal.ID
		}
		return values[i].observedAt.Before(values[j].observedAt)
	})
	out := make([]model.Signal, 0, len(values))
	for _, item := range values {
		out = append(out, item.signal)
	}
	return out
}

func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now().UTC())
	return len(s.entries)
}

func (s *Store) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.ttl)
	for s.expiry.Len() > 0 {
		candidate := s.expiry[0]
		current, exists := s.entries[candidate.id]
		if !exists || current.generation != candidate.generation {
			heap.Pop(&s.expiry)
			continue
		}

		expired := current.observedAt.Before(cutoff)
		overCapacity := len(s.entries) > s.maxEntries
		if !expired && !overCapacity {
			break
		}
		heap.Pop(&s.expiry)
		delete(s.entries, candidate.id)
	}
}
