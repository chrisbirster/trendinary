package scanner

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

// SourceStatus is the operational view of one public discovery adapter.
type SourceStatus struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Kind          string        `json:"kind"`
	Policy        string        `json:"policy,omitempty"`
	URL           string        `json:"url,omitempty"`
	TermsURL      string        `json:"terms_url,omitempty"`
	Enabled       bool          `json:"enabled"`
	Cadence       time.Duration `json:"cadence"`
	LastAttemptAt time.Time     `json:"last_attempt_at,omitempty"`
	LastSuccessAt time.Time     `json:"last_success_at,omitempty"`
	NextRunAt     time.Time     `json:"next_run_at,omitempty"`
	LastError     string        `json:"last_error,omitempty"`
	Failures      int           `json:"failures"`
	CachedSignals int           `json:"cached_signals"`
}

type SourceMetadata struct {
	ID               string
	Name             string
	Kind             string
	Policy           string
	URL              string
	TermsURL         string
	Cadence          time.Duration
	StartImmediately bool
}

type SourceStatusProvider interface {
	SourceStatus() SourceStatus
}

// ScheduledSource lets the scanner keep its fast global tick while respecting
// source-specific polling limits. Between polls it returns the last good batch.
type ScheduledSource struct {
	mu       sync.Mutex
	source   DiscoverySource
	meta     SourceMetadata
	now      func() time.Time
	cached   []model.Signal
	attempt  time.Time
	success  time.Time
	next     time.Time
	lastErr  string
	failures int
}

func NewScheduledSource(source DiscoverySource, meta SourceMetadata) *ScheduledSource {
	if meta.Cadence <= 0 {
		meta.Cadence = 15 * time.Minute
	}
	if meta.ID == "" {
		meta.ID = source.Name()
	}
	if meta.Name == "" {
		meta.Name = source.Name()
	}
	now := time.Now().UTC()
	next := time.Time{}
	if !meta.StartImmediately {
		next = now.Add(initialSourceDelay(meta.ID, meta.Cadence))
	}
	return &ScheduledSource{source: source, meta: meta, now: func() time.Time { return time.Now().UTC() }, next: next}
}

func (s *ScheduledSource) Name() string { return s.meta.Name }

func (s *ScheduledSource) Discover(ctx context.Context) ([]model.Signal, error) {
	if s == nil || s.source == nil {
		return nil, nil
	}
	now := s.now().UTC()
	s.mu.Lock()
	if !s.next.IsZero() && now.Before(s.next) {
		cached := cloneScheduledSignals(s.cached)
		s.mu.Unlock()
		return cached, nil
	}
	s.attempt = now
	s.mu.Unlock()

	values, err := s.source.Discover(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.failures++
		s.lastErr = err.Error()
		delay := s.meta.Cadence
		for i := 1; i < s.failures && i < 6; i++ {
			delay *= 2
		}
		if delay > 12*time.Hour {
			delay = 12 * time.Hour
		}
		s.next = now.Add(delay + sourceJitter(s.meta.ID, delay))
		return cloneScheduledSignals(s.cached), err
	}
	s.cached = cloneScheduledSignals(values)
	s.success = now
	s.lastErr = ""
	s.failures = 0
	s.next = now.Add(s.meta.Cadence + sourceJitter(s.meta.ID, s.meta.Cadence))
	return cloneScheduledSignals(s.cached), nil
}

func (s *ScheduledSource) SourceStatus() SourceStatus {
	if s == nil {
		return SourceStatus{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return SourceStatus{
		ID: s.meta.ID, Name: s.meta.Name, Kind: s.meta.Kind, Policy: s.meta.Policy,
		URL: s.meta.URL, TermsURL: s.meta.TermsURL, Enabled: s.source != nil,
		Cadence: s.meta.Cadence, LastAttemptAt: s.attempt, LastSuccessAt: s.success,
		NextRunAt: s.next, LastError: s.lastErr, Failures: s.failures, CachedSignals: len(s.cached),
	}
}

func SourceStatuses(sources []DiscoverySource) []SourceStatus {
	out := make([]SourceStatus, 0, len(sources))
	for _, source := range sources {
		if provider, ok := source.(SourceStatusProvider); ok {
			out = append(out, provider.SourceStatus())
		}
	}
	return out
}

func sourceHash(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

func initialSourceDelay(id string, cadence time.Duration) time.Duration {
	if cadence <= 0 {
		return 0
	}
	window := cadence
	if window > 10*time.Minute {
		window = 10 * time.Minute
	}
	if window < time.Minute {
		window = time.Minute
	}
	fraction := float64(sourceHash(id)%10001) / 10000
	return time.Duration(float64(window) * fraction)
}

func sourceJitter(id string, cadence time.Duration) time.Duration {
	if cadence <= 0 {
		return 0
	}
	// Stable 0-10% positive jitter prevents every source with the same cadence
	// from drifting back into lockstep after a long-running process.
	fraction := float64(sourceHash(id)%1001) / 10000
	return time.Duration(float64(cadence) * fraction)
}

func cloneScheduledSignals(values []model.Signal) []model.Signal {
	out := make([]model.Signal, len(values))
	copy(out, values)
	return out
}
