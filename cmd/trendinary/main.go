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
	"github.com/chrisbirster/trendinary/internal/sourceregistry"
	"github.com/chrisbirster/trendinary/internal/store"
	webapp "github.com/chrisbirster/trendinary/internal/web"
)

func main() {
	historical, storageBackend, err := openHistory()
	if err != nil {
		slog.Error("open history database", "backend", storageBackend, "error", err)
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

	discoverySources, inactiveSources := buildDiscoverySources(historical)
	jetstreamEnabled := os.Getenv("TRENDINARY_JETSTREAM_DISABLED") != "1"
	scannerEnabled := os.Getenv("TRENDINARY_SCANNER_DISABLED") != "1"
	scanInterval := envDuration("TRENDINARY_SCAN_INTERVAL", 2*time.Minute)
	runtimeStatus := runtimeinfo.New(jetstreamEnabled, scannerEnabled)
	runtimeStatus.SetSources(runtimeSourceSnapshots(discoverySources, inactiveSources, scanInterval, jetstreamEnabled, scannerEnabled))

	publicHandler := httpapi.New(
		memory,
		webapp.Handler(),
		httpapi.WithHistory(historical),
		httpapi.WithRuntime(runtimeStatus, streamWindow),
	)
	adminHandler := httpapi.NewAdmin(publicHandler, editorialService, os.Getenv("TRENDINARY_ADMIN_PASSWORD"))
	handler := httpapi.NewIntegration(adminHandler, editorialService, os.Getenv("TRENDINARY_INTEGRATION_TOKEN"))
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
		go runScanner(ctx, scan, streamWindow, discoverySources, inactiveSources, scanInterval, runtimeStatus, jetstreamEnabled, scannerEnabled)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("trendinary listening", "addr", server.Addr, "storage", storageBackend, "discovery_sources", len(discoverySources)+2, "admin_enabled", os.Getenv("TRENDINARY_ADMIN_PASSWORD") != "", "integration_enabled", os.Getenv("TRENDINARY_INTEGRATION_TOKEN") != "")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func buildDiscoverySources(historical *history.Store) ([]scanner.DiscoverySource, []runtimeinfo.SourceSnapshot) {
	out := make([]scanner.DiscoverySource, 0, 64)
	configured := map[string]bool{}
	add := func(source scanner.DiscoverySource, meta scanner.SourceMetadata) {
		out = append(out, scanner.NewScheduledSource(source, meta))
		configured[meta.ID] = true
	}

	if os.Getenv("TRENDINARY_GITHUB_DISABLED") != "1" {
		add(githubdiscovery.New(nil, os.Getenv("TRENDINARY_GITHUB_TOKEN"), envInt("TRENDINARY_GITHUB_LIMIT", 40)), scanner.SourceMetadata{
			ID: "github-api", Name: "GitHub repository discovery", Kind: "api", Policy: "official-api",
			URL: "https://api.github.com/search/repositories", TermsURL: "https://docs.github.com/en/rest/search/search",
			Cadence: 15 * time.Minute, StartImmediately: true,
		})
	}

	if os.Getenv("TRENDINARY_RSS_DISABLED") != "1" {
		for _, entry := range sourceregistry.EnabledRSS() {
			add(rss.NewFeed(nil, entry.Name, entry.URL, envInt("TRENDINARY_RSS_FEED_LIMIT", 20)), scanner.SourceMetadata{
				ID: entry.ID, Name: entry.Name, Kind: entry.Kind, Policy: string(entry.Policy), URL: entry.URL,
				TermsURL: entry.TermsURL, Cadence: entry.Cadence,
			})
		}
		for index, feed := range splitCSV(os.Getenv("TRENDINARY_RSS_FEEDS")) {
			id := fmt.Sprintf("custom-rss-%d", index+1)
			add(rss.NewFeed(nil, id, feed, envInt("TRENDINARY_RSS_FEED_LIMIT", 20)), scanner.SourceMetadata{
				ID: id, Name: "Custom RSS · " + feed, Kind: "rss", Policy: "operator-configured", URL: feed,
				Cadence: envDuration("TRENDINARY_CUSTOM_RSS_CADENCE", 30*time.Minute),
			})
		}
	}

	if os.Getenv("TRENDINARY_WIKIPEDIA_DISABLED") != "1" {
		add(wikipedia.New(nil, envInt("TRENDINARY_WIKIPEDIA_LIMIT", 40)), scanner.SourceMetadata{
			ID: "wikipedia", Name: "Wikipedia pageviews", Kind: "api", Policy: "official-api",
			URL: "https://wikimedia.org/api/rest_v1/metrics/pageviews/top/", TermsURL: "https://wikimedia.org/api/rest_v1/",
			Cadence: 6 * time.Hour, StartImmediately: true,
		})
	}

	if os.Getenv("TRENDINARY_GDELT_DISABLED") != "1" {
		add(gdelt.New(nil, os.Getenv("TRENDINARY_GDELT_QUERY"), envInt("TRENDINARY_GDELT_LIMIT", 250)), scanner.SourceMetadata{
			ID: "gdelt", Name: "GDELT", Kind: "api", Policy: "official-api",
			URL: "https://api.gdeltproject.org/api/v2/doc/doc", TermsURL: "https://www.gdeltproject.org/",
			Cadence: 10 * time.Minute, StartImmediately: true,
		})
	}

	if os.Getenv("TRENDINARY_YOUTUBE_DISABLED") != "1" {
		if key := strings.TrimSpace(os.Getenv("TRENDINARY_YOUTUBE_API_KEY")); key != "" {
			add(youtube.New(nil, key, envString("TRENDINARY_YOUTUBE_REGION", "US"), envInt("TRENDINARY_YOUTUBE_LIMIT", 25)), scanner.SourceMetadata{
				ID: "youtube-api", Name: "YouTube Data API", Kind: "api", Policy: "official-api",
				URL: "https://www.googleapis.com/youtube/v3/", TermsURL: "https://developers.google.com/youtube/v3/",
				Cadence: time.Hour,
			})
		} else {
			slog.Info("YouTube discovery disabled because TRENDINARY_YOUTUBE_API_KEY is not configured")
		}
	}

	if os.Getenv("TRENDINARY_NEWSDATA_DISABLED") != "1" {
		if key := strings.TrimSpace(os.Getenv("TRENDINARY_NEWSDATA_API_KEY")); key != "" {
			add(newsdata.New(nil, historical, historical, key, envInt("TRENDINARY_NEWSDATA_DAILY_CALLS", 200), os.Getenv("TRENDINARY_NEWSDATA_CATEGORY")), scanner.SourceMetadata{
				ID: "newsdata-api", Name: "NewsData", Kind: "api", Policy: "official-api",
				URL: "https://newsdata.io/api/1/latest", TermsURL: "https://newsdata.io/documentation", Cadence: 2 * time.Hour,
			})
		} else {
			slog.Info("NewsData discovery disabled because TRENDINARY_NEWSDATA_API_KEY is not configured")
		}
	}

	inactive := make([]runtimeinfo.SourceSnapshot, 0)
	for _, entry := range sourceregistry.Default() {
		if entry.Enabled || configured[entry.ID] {
			continue
		}
		inactive = append(inactive, runtimeinfo.SourceSnapshot{
			ID: entry.ID, Name: entry.Name, Kind: entry.Kind, Policy: string(entry.Policy), URL: entry.URL,
			TermsURL: entry.TermsURL, Enabled: false, Cadence: entry.Cadence,
		})
	}
	return out, inactive
}

func runtimeSourceSnapshots(sources []scanner.DiscoverySource, inactive []runtimeinfo.SourceSnapshot, scanInterval time.Duration, jetstreamEnabled, scannerEnabled bool) []runtimeinfo.SourceSnapshot {
	out := make([]runtimeinfo.SourceSnapshot, 0, len(sources)+len(inactive)+2)
	out = append(out,
		runtimeinfo.SourceSnapshot{ID: "hacker-news", Name: "Hacker News", Kind: "api", Policy: "official-api", URL: "https://hacker-news.firebaseio.com/", Enabled: scannerEnabled, Cadence: scanInterval},
		runtimeinfo.SourceSnapshot{ID: "bluesky-jetstream", Name: "Bluesky Jetstream", Kind: "stream", Policy: "public-stream", URL: envString("TRENDINARY_JETSTREAM_HOST", "https://jetstream.us-east.bsky.network"), Enabled: jetstreamEnabled},
	)
	for _, value := range scanner.SourceStatuses(sources) {
		out = append(out, runtimeinfo.SourceSnapshot{
			ID: value.ID, Name: value.Name, Kind: value.Kind, Policy: value.Policy, URL: value.URL, TermsURL: value.TermsURL,
			Enabled: value.Enabled, Cadence: value.Cadence, LastAttemptAt: value.LastAttemptAt, LastSuccessAt: value.LastSuccessAt,
			NextRunAt: value.NextRunAt, LastError: value.LastError, Failures: value.Failures, CachedSignals: value.CachedSignals,
		})
	}
	out = append(out, inactive...)
	return out
}

func runCommand(ctx context.Context, historical *history.Store, editorialService *editorial.Service, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "ingest":
		if len(args) != 2 {
			return true, fmt.Errorf("usage: trendinary ingest <source>")
		}
		run, err := editorialService.Ingest(ctx, args[1])
		_ = json.NewEncoder(os.Stdout).Encode(run)
		return true, err
	case "backup":
		if len(args) != 2 {
			return true, fmt.Errorf("usage: trendinary backup <destination.db>")
		}
		return true, historical.Backup(ctx, args[1])
	case "calibration":
		calibration, err := historical.Calibration(ctx, time.Now().UTC().Add(-30*24*time.Hour))
		if err == nil {
			_ = json.NewEncoder(os.Stdout).Encode(calibration)
		}
		return true, err
	default:
		return false, nil
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

func runScanner(ctx context.Context, scan *scanner.Scanner, streamWindow *recent.Store, sources []scanner.DiscoverySource, inactive []runtimeinfo.SourceSnapshot, interval time.Duration, status *runtimeinfo.Status, jetstreamEnabled, scannerEnabled bool) {
	run := func() {
		status.ScanStarted(time.Now().UTC())
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		result, err := scan.RunWithSourcesV2(runCtx, streamWindow, sources)
		status.SetSources(runtimeSourceSnapshots(sources, inactive, interval, jetstreamEnabled, scannerEnabled))
		if err != nil {
			status.ScanFailed(err)
			slog.Warn("trend scan failed", "error", err)
			return
		}
		status.ScanSucceeded(time.Now().UTC(), result.Signals, result.Clusters, result.Trends, len(result.Warnings))
		slog.Info("trend scan complete", "signals", result.Signals, "stream_window", streamWindow.Len(), "clusters", result.Clusters, "trends", result.Trends, "warnings", len(result.Warnings))
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

func splitCSV(value string) []string {
	out := []string{}
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
