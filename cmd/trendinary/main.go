package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	githubdiscovery "github.com/chrisbirster/trendinary/internal/ingest/github"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	jetstreaming "github.com/chrisbirster/trendinary/internal/ingest/jetstream"
	"github.com/chrisbirster/trendinary/internal/ingest/reddit"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
	"github.com/chrisbirster/trendinary/internal/scanner"
	"github.com/chrisbirster/trendinary/internal/store"
	webapp "github.com/chrisbirster/trendinary/internal/web"
)

func main() {
	port := envString("PORT", "8080")
	dbPath := envString("TRENDINARY_DB_PATH", "trendinary.db")

	historical, err := history.Open(dbPath)
	if err != nil {
		slog.Error("open history database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer historical.Close()

	memory := store.NewMemory()
	streamWindow := recent.New(
		envInt("TRENDINARY_RECENT_SIGNAL_LIMIT", 50_000),
		envDuration("TRENDINARY_RECENT_SIGNAL_TTL", 30*time.Minute),
	)
	blueskyClient := bluesky.NewClient(nil)
	scan := scanner.New(
		hackernews.NewClient(nil),
		blueskyClient,
		historical,
		memory,
		scanner.Config{
			HackerNewsLimit:     envInt("TRENDINARY_HN_LIMIT", 30),
			EnrichClusters:      envInt("TRENDINARY_ENRICH_CLUSTERS", 8),
			BlueskyLimit:        envInt("TRENDINARY_BLUESKY_LIMIT", 20),
			PublishedTrendLimit: envInt("TRENDINARY_TREND_LIMIT", 20),
		},
	)

	discoverySources := make([]scanner.DiscoverySource, 0, 2)
	if os.Getenv("TRENDINARY_GITHUB_DISABLED") != "1" {
		discoverySources = append(discoverySources, githubdiscovery.New(
			nil,
			os.Getenv("TRENDINARY_GITHUB_TOKEN"),
			envInt("TRENDINARY_GITHUB_LIMIT", 40),
		))
	}
	if os.Getenv("TRENDINARY_REDDIT_DISABLED") != "1" {
		clientID := stringsTrim(os.Getenv("TRENDINARY_REDDIT_CLIENT_ID"))
		clientSecret := stringsTrim(os.Getenv("TRENDINARY_REDDIT_CLIENT_SECRET"))
		if clientID != "" && clientSecret != "" {
			discoverySources = append(discoverySources, reddit.New(
				nil,
				clientID,
				clientSecret,
				envString("TRENDINARY_REDDIT_USER_AGENT", "trendinary/0.1 (+https://trendinary.com)"),
				envInt("TRENDINARY_REDDIT_LIMIT", 50),
			))
		} else {
			slog.Info("reddit discovery disabled because OAuth credentials are not configured")
		}
	}

	jetstreamEnabled := os.Getenv("TRENDINARY_JETSTREAM_DISABLED") != "1"
	scannerEnabled := os.Getenv("TRENDINARY_SCANNER_DISABLED") != "1"
	runtimeStatus := runtimeinfo.New(jetstreamEnabled, scannerEnabled)

	handler := httpapi.New(
		memory,
		webapp.Handler(),
		httpapi.WithHistory(historical),
		httpapi.WithRuntime(runtimeStatus, streamWindow),
	)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if jetstreamEnabled {
		collector := jetstreaming.New(
			historical,
			streamWindow,
			jetstreaming.Config{
				Host:      envString("TRENDINARY_JETSTREAM_HOST", "https://jetstream.us-east.bsky.network"),
				BatchSize: envInt("TRENDINARY_JETSTREAM_BATCH_SIZE", 128),
				Status:    runtimeStatus,
			},
		)
		go runJetstream(ctx, collector, runtimeStatus)
	}

	if scannerEnabled {
		interval := envDuration("TRENDINARY_SCAN_INTERVAL", 2*time.Minute)
		go runScanner(ctx, scan, streamWindow, discoverySources, interval, runtimeStatus)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("trendinary listening", "addr", server.Addr, "db", dbPath, "discovery_sources", len(discoverySources)+2)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func runJetstream(ctx context.Context, collector *jetstreaming.Collector, status *runtimeinfo.Status) {
	backoff := 2 * time.Second
	for {
		err := collector.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, jetstreaming.ErrFatal) {
			status.StreamFatal(err)
			slog.Error("jetstream collector stopped after fatal stream error", "error", err)
			return
		}
		status.StreamDisconnected(err, backoff)
		slog.Warn("jetstream collector disconnected", "error", err, "retry_in", backoff)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff *= 2
		if backoff > time.Minute {
			backoff = time.Minute
		}
	}
}

func runScanner(ctx context.Context, scan *scanner.Scanner, streamWindow *recent.Store, sources []scanner.DiscoverySource, interval time.Duration, status *runtimeinfo.Status) {
	run := func() {
		status.ScanStarted(time.Now().UTC())
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		result, err := scan.RunWithSources(runCtx, streamWindow, sources)
		if err != nil {
			status.ScanFailed(err)
			slog.Warn("trend scan failed", "error", err)
			return
		}
		status.ScanSucceeded(time.Now().UTC(), result.Signals, result.Clusters, result.Trends, len(result.Warnings))
		slog.Info("trend scan complete",
			"signals", result.Signals,
			"stream_window", streamWindow.Len(),
			"clusters", result.Clusters,
			"trends", result.Trends,
			"warnings", len(result.Warnings),
		)
		for _, warning := range result.Warnings {
			slog.Debug("trend scan warning", "warning", warning)
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func envString(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		slog.Warn("invalid integer environment variable", "name", name, "value", value, "fallback", fallback)
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 15*time.Second {
		slog.Warn("invalid duration environment variable", "name", name, "value", value, "fallback", fallback)
		return fallback
	}
	return parsed
}

func stringsTrim(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t' || value[0] == '\n' || value[0] == '\r') {
		value = value[1:]
	}
	for len(value) > 0 {
		last := value[len(value)-1]
		if last != ' ' && last != '\t' && last != '\n' && last != '\r' {
			break
		}
		value = value[:len(value)-1]
	}
	return value
}
