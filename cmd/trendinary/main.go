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
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	jetstreaming "github.com/chrisbirster/trendinary/internal/ingest/jetstream"
	"github.com/chrisbirster/trendinary/internal/recent"
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
	scan := scanner.New(
		hackernews.NewClient(nil),
		bluesky.NewClient(nil),
		historical,
		memory,
		scanner.Config{
			HackerNewsLimit: envInt("TRENDINARY_HN_LIMIT", 30),
			EnrichClusters: envInt("TRENDINARY_ENRICH_CLUSTERS", 8),
			BlueskyLimit: envInt("TRENDINARY_BLUESKY_LIMIT", 20),
			PublishedTrendLimit: envInt("TRENDINARY_TREND_LIMIT", 20),
		},
	)

	handler := httpapi.New(memory, webapp.Handler(), httpapi.WithHistory(historical))
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

	if os.Getenv("TRENDINARY_JETSTREAM_DISABLED") != "1" {
		collector := jetstreaming.New(
			historical,
			streamWindow,
			jetstreaming.Config{
				Host: envString("TRENDINARY_JETSTREAM_HOST", "https://jetstream.us-east.bsky.network"),
				BatchSize: envInt("TRENDINARY_JETSTREAM_BATCH_SIZE", 128),
			},
		)
		go runJetstream(ctx, collector)
	}

	if os.Getenv("TRENDINARY_SCANNER_DISABLED") != "1" {
		interval := envDuration("TRENDINARY_SCAN_INTERVAL", 2*time.Minute)
		go runScanner(ctx, scan, streamWindow, interval)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("trendinary listening", "addr", server.Addr, "db", dbPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func runJetstream(ctx context.Context, collector *jetstreaming.Collector) {
	backoff := 2 * time.Second
	for {
		err := collector.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, jetstreaming.ErrFatal) {
			slog.Error("jetstream collector stopped after fatal stream error", "error", err)
			return
		}
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

func runScanner(ctx context.Context, scan *scanner.Scanner, streamWindow *recent.Store, interval time.Duration) {
	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		result, err := scan.RunWithRecent(runCtx, streamWindow)
		if err != nil {
			slog.Warn("trend scan failed", "error", err)
			return
		}
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
