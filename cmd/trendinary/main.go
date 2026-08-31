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

	if os.Getenv("TRENDINARY_SCANNER_DISABLED") != "1" {
		interval := envDuration("TRENDINARY_SCAN_INTERVAL", 2*time.Minute)
		go runScanner(ctx, scan, interval)
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

func runScanner(ctx context.Context, scan *scanner.Scanner, interval time.Duration) {
	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		result, err := scan.RunOnce(runCtx)
		if err != nil {
			slog.Warn("trend scan failed", "error", err)
			return
		}
		slog.Info("trend scan complete",
			"signals", result.Signals,
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
