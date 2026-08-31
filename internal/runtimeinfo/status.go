package runtimeinfo

import (
	"sync"
	"time"
)

type StreamSnapshot struct {
	Enabled       bool          `json:"enabled"`
	Connected     bool          `json:"connected"`
	Host          string        `json:"host,omitempty"`
	LastEventAt   time.Time     `json:"last_event_at,omitempty"`
	LastCursor    uint64        `json:"last_cursor,omitempty"`
	LastBatchSize int           `json:"last_batch_size,omitempty"`
	LastError     string        `json:"last_error,omitempty"`
	RetryIn       time.Duration `json:"retry_in,omitempty"`
	Reconnects    int           `json:"reconnects"`
}

type ScannerSnapshot struct {
	Enabled       bool      `json:"enabled"`
	Running       bool      `json:"running"`
	LastStartedAt time.Time `json:"last_started_at,omitempty"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	Signals       int       `json:"signals"`
	Clusters      int       `json:"clusters"`
	Trends        int       `json:"trends"`
	Warnings      int       `json:"warnings"`
}

type Snapshot struct {
	Stream  StreamSnapshot  `json:"stream"`
	Scanner ScannerSnapshot `json:"scanner"`
}

type Status struct {
	mu      sync.RWMutex
	stream  StreamSnapshot
	scanner ScannerSnapshot
}

func New(streamEnabled, scannerEnabled bool) *Status {
	return &Status{
		stream:  StreamSnapshot{Enabled: streamEnabled},
		scanner: ScannerSnapshot{Enabled: scannerEnabled},
	}
}

func (s *Status) StreamConnecting(host string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stream.Host = host
	s.stream.Connected = false
	s.stream.RetryIn = 0
	s.mu.Unlock()
}

func (s *Status) StreamConnected(host string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stream.Host = host
	s.stream.Connected = true
	s.stream.LastError = ""
	s.stream.RetryIn = 0
	s.mu.Unlock()
}

func (s *Status) StreamBatch(cursor uint64, batchSize int, observedAt time.Time) {
	if s == nil {
		return
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	s.mu.Lock()
	s.stream.Connected = true
	s.stream.LastEventAt = observedAt.UTC()
	s.stream.LastCursor = cursor
	s.stream.LastBatchSize = batchSize
	s.mu.Unlock()
}

func (s *Status) StreamDisconnected(err error, retryIn time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stream.Connected = false
	s.stream.RetryIn = retryIn
	s.stream.Reconnects++
	if err != nil {
		s.stream.LastError = err.Error()
	}
	s.mu.Unlock()
}

func (s *Status) StreamFatal(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stream.Connected = false
	s.stream.RetryIn = 0
	if err != nil {
		s.stream.LastError = err.Error()
	}
	s.mu.Unlock()
}

func (s *Status) ScanStarted(at time.Time) {
	if s == nil {
		return
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	s.mu.Lock()
	s.scanner.Running = true
	s.scanner.LastStartedAt = at.UTC()
	s.mu.Unlock()
}

func (s *Status) ScanSucceeded(at time.Time, signals, clusters, trends, warnings int) {
	if s == nil {
		return
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	s.mu.Lock()
	s.scanner.Running = false
	s.scanner.LastSuccessAt = at.UTC()
	s.scanner.LastError = ""
	s.scanner.Signals = signals
	s.scanner.Clusters = clusters
	s.scanner.Trends = trends
	s.scanner.Warnings = warnings
	s.mu.Unlock()
}

func (s *Status) ScanFailed(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.scanner.Running = false
	if err != nil {
		s.scanner.LastError = err.Error()
	}
	s.mu.Unlock()
}

func (s *Status) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{Stream: s.stream, Scanner: s.scanner}
}
