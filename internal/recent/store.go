package recent

import (
	"sort"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type entry struct {
	signal     model.Signal
	observedAt time.Time
}

// Store is a bounded, concurrency-safe window of recently observed signals.
// It is intentionally not the durable source of truth; SQLite owns durability.
// This buffer exists so the scoring scanner can cheaply cluster the current
// attention window without loading the full history database every run.
type Store struct {
	mu         sync.RWMutex
	entries    map[string]entry
	maxEntries int
	ttl        time.Duration
}

func New(maxEntries int, ttl time.Duration) *Store {
	if maxEntries <= 0 {
		maxEntries = 50_000
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &Store{
		entries:    make(map[string]entry, min(maxEntries, 4096)),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

func (s *Store) Upsert(signal model.Signal, observedAt time.Time) {
	if signal.ID == "" {
		return
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	observedAt = observedAt.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[signal.ID] = entry{signal: signal, observedAt: observedAt}
	s.pruneLocked(observedAt)
}

func (s *Store) UpsertMany(values []model.Signal, observedAt time.Time) {
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	observedAt = observedAt.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, signal := range values {
		if signal.ID == "" {
			continue
		}
		s.entries[signal.ID] = entry{signal: signal, observedAt: observedAt}
	}
	s.pruneLocked(observedAt)
}

func (s *Store) Delete(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	delete(s.entries, id)
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *Store) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.ttl)
	for id, item := range s.entries {
		if item.observedAt.Before(cutoff) {
			delete(s.entries, id)
		}
	}
	if len(s.entries) <= s.maxEntries {
		return
	}

	values := make([]struct {
		id string
		entry
	}, 0, len(s.entries))
	for id, item := range s.entries {
		values = append(values, struct {
			id string
			entry
		}{id: id, entry: item})
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].observedAt.Equal(values[j].observedAt) {
			return values[i].id < values[j].id
		}
		return values[i].observedAt.Before(values[j].observedAt)
	})
	remove := len(values) - s.maxEntries
	for index := 0; index < remove; index++ {
		delete(s.entries, values[index].id)
	}
}
