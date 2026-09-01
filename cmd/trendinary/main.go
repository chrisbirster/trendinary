package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chrisbirster/trendinary/internal/editorial"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/ingest/gdelt"
	githubdiscovery "github.com/chrisbirster/trendinary/internal/ingest/github"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	jetstreaming "github.com/chrisbirster/trendinary/internal/ingest/jetstream"
	"github.com/chrisbirster/trendinary/internal/ingest/newsdata"
	"github.com/chrisbirster/trendinary/internal/ingest/rss"
	"github.com/chrisbirster/trendinary/internal/ingest/wikipedia"
	"github.com/chrisbirster/trendinary/internal/ingest/youtube"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
	"github.com/chrisbirster/trendinary/internal/scanner"
	"github.com/chrisbirster/trendinary/internal/store"
	webapp "github.com/chrisbirster/trendinary/internal/web"
)

func main() {
	dbPath := envString("TRENDINARY_DB_PATH", "trendinary.db")
	historical, err := history.Open(dbPath)
	if err != nil {
		slog.Error("open history database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer historical.Close()

	editorialStore, err := editorial.NewStore(historical.DB())
	if err != nil {
		slog.Error("open editorial database", "error", err)
		os.Exit(1)
	}
	techURLs := editorial.NewTechURLs(nil)
	if archiveDir := strings.TrimSpace(os.Getenv("TRENDINARY_RAW_ARCHIVE_DIR")); archiveDir != "" {
		techURLs.SetArchiveDir(archiveDir)
	}
	editorialService, err := editorial.NewService(editorialStore, nil, techURLs)
	if err != nil {
		slog.Error("configure editorial service", "error", err)
		os.Exit(1)
	}

	if handled, err := runCommand(context.Background(), historical, editorialService, os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	port := envString("PORT", "8080")
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
			SourceUniverse:      envInt("TRENDINARY_SOURCE_UNIVERSE", 8),
		},
	)

	discoverySources := buildDiscoverySources(historical)
	jetstreamEnabled := os.Getenv("TRENDINARY_JETSTREAM_DISABLED") != "1"
	scannerEnabled := os.Getenv("TRENDINARY_SCANNER_DISABLED") != "1"
	runtimeStatus := runtimeinfo.New(jetstreamEnabled, scannerEnabled)

	publicHandler := httpapi.New(
		memory,
		webapp.Handler(),
		httpapi.WithHistory(historical),
		httpapi.WithRuntime(runtimeStatus, streamWindow),
	)
	handler := httpapi.NewAdmin(publicHandler, editorialService, os.Getenv("TRENDINARY_ADMIN_PASSWORD"))
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      60 * time.Second,
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

	slog.Info("trendinary listening", "addr", server.Addr, "db", dbPath, "discovery_sources", len(discoverySources)+2, "admin_enabled", os.Getenv("TRENDINARY_ADMIN_PASSWORD") != "")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func buildDiscoverySources(historical *history.Store) []scanner.DiscoverySource {
	out := make([]scanner.DiscoverySource, 0, 6)
	if os.Getenv("TRENDINARY_GITHUB_DISABLED") != "1" {
		out = append(out, githubdiscovery.New(nil, os.Getenv("TRENDINARY_GITHUB_TOKEN"), envInt("TRENDINARY_GITHUB_LIMIT", 40)))
	}
	if os.Getenv("TRENDINARY_RSS_DISABLED") != "1" {
		if feeds := splitCSV(os.Getenv("TRENDINARY_RSS_FEEDS")); len(feeds) > 0 {
			out = append(out, rss.New(nil, feeds, envInt("TRENDINARY_RSS_LIMIT", 50)))
		}
	}
	if os.Getenv("TRENDINARY_WIKIPEDIA_DISABLED") != "1" {
		out = append(out, wikipedia.New(nil, envInt("TRENDINARY_WIKIPEDIA_LIMIT", 40)))
	}
	if os.Getenv("TRENDINARY_YOUTUBE_DISABLED") != "1" {
		if key := strings.TrimSpace(os.Getenv("TRENDINARY_YOUTUBE_API_KEY")); key != "" {
			out = append(out, youtube.New(nil, key, envString("TRENDINARY_YOUTUBE_REGION", "US"), envInt("TRENDINARY_YOUTUBE_LIMIT", 25)))
		}
	}
	if os.Getenv("TRENDINARY_GDELT_DISABLED") != "1" {
		out = append(out, gdelt.New(nil, os.Getenv("TRENDINARY_GDELT_QUERY"), envInt("TRENDINARY_GDELT_LIMIT", 250)))
	}
	if os.Getenv("TRENDINARY_NEWSDATA_DISABLED") != "1" {
		if key := strings.TrimSpace(os.Getenv("TRENDINARY_NEWSDATA_API_KEY")); key != "" {
			out = append(out, newsdata.New(
				nil,
				historical,
				historical,
				key,
				envInt("TRENDINARY_NEWSDATA_DAILY_CALLS", 200),
				os.Getenv("TRENDINARY_NEWSDATA_CATEGORY"),
			))
		} else {
			slog.Info("NewsData discovery disabled because TRENDINARY_NEWSDATA_API_KEY is not configured")
		}
	}
	return out
}

func runCommand(ctx context.Context, historical *history.Store, editorialService *editorial.Service, args []string) (bool, error) {
	if len(args) == 0 { return false, nil }
	switch args[0] {
	case "ingest":
		if len(args) != 2 { return true, fmt.Errorf("usage: trendinary ingest <source>") }
		run, err := editorialService.Ingest(ctx, args[1])
		_ = json.NewEncoder(os.Stdout).Encode(run)
		return true, err
	case "backup":
		if len(args) != 2 { return true, fmt.Errorf("usage: trendinary backup <destination.db>") }
		return true, historical.Backup(ctx, args[1])
	case "calibration":
		calibration, err := historical.Calibration(ctx, time.Now().UTC().Add(-30*24*time.Hour))
		if err == nil { _ = json.NewEncoder(os.Stdout).Encode(calibration) }
		return true, err
	default:
		return false, nil
	}
}

func runJetstream(ctx context.Context, collector *jetstreaming.Collector, status *runtimeinfo.Status) {
	backoff := 2 * time.Second
	for {
		err := collector.Run(ctx)
		if ctx.Err() != nil { return }
		if errors.Is(err, jetstreaming.ErrFatal) {
			status.StreamFatal(err)
			slog.Error("jetstream collector stopped after fatal stream error", "error", err)
			return
		}
		status.StreamDisconnected(err, backoff)
		slog.Warn("jetstream collector disconnected", "error", err, "retry_in", backoff)
		timer := time.NewTimer(backoff)
		select { case <-ctx.Done(): timer.Stop(); return; case <-timer.C: }
		backoff *= 2
		if backoff > time.Minute { backoff = time.Minute }
	}
}

func runScanner(ctx context.Context, scan *scanner.Scanner, streamWindow *recent.Store, sources []scanner.DiscoverySource, interval time.Duration, status *runtimeinfo.Status) {
	run := func() {
		status.ScanStarted(time.Now().UTC())
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		result, err := scan.RunWithSourcesV2(runCtx, streamWindow, sources)
		if err != nil {
			status.ScanFailed(err)
			slog.Warn("trend scan failed", "error", err)
			return
		}
		status.ScanSucceeded(time.Now().UTC(), result.Signals, result.Clusters, result.Trends, len(result.Warnings))
		slog.Info("trend scan complete", "signals", result.Signals, "stream_window", streamWindow.Len(), "clusters", result.Clusters, "trends", result.Trends, "warnings", len(result.Warnings))
		for _, warning := range result.Warnings { slog.Debug("trend scan warning", "warning", warning) }
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for { select { case <-ctx.Done(): return; case <-ticker.C: run() } }
}

func envString(name, fallback string) string { if value := os.Getenv(name); value != "" { return value }; return fallback }
func envInt(name string, fallback int) int {
	value := os.Getenv(name); if value == "" { return fallback }
	parsed, err := strconv.Atoi(value); if err != nil || parsed <= 0 { slog.Warn("invalid integer environment variable", "name", name, "value", value, "fallback", fallback); return fallback }
	return parsed
}
func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name); if value == "" { return fallback }
	parsed, err := time.ParseDuration(value); if err != nil || parsed < 15*time.Second { slog.Warn("invalid duration environment variable", "name", name, "value", value, "fallback", fallback); return fallback }
	return parsed
}
func splitCSV(value string) []string { out:=[]string{}; for _,part:=range strings.Split(value,","){if trimmed:=strings.TrimSpace(part);trimmed!=""{out=append(out,trimmed)}};return out }
