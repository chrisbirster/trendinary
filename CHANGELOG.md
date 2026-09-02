# Changelog

All notable Trendinary changes are tracked here. Releases use Semantic Versioning and Git tags prefixed with `v`.

## [Unreleased]

## [0.3.0] - 2026-09-02

### Added

- Signal Quality v3 private `/admin/quality` calibration workspace with durable labels for real trends, noise, duplicates, early/late timing, bad clusters, and canonical-name mistakes.
- Exact persisted signal-to-stable-trend memberships so production human labels can replay the observations that actually formed each detected trend.
- Human-label quality reporting for Top-10 and Top-25 precision, precision proxy, false-positive rate, duplicate-cluster rate, early-hit rate, cluster health, naming health, source breadth, lifecycle timing, score separation, and a conservative publication-threshold recommendation.
- Deterministic production replay benchmark that feeds persisted labeled signal memberships back through the entity-aware clusterer and reports pairwise precision/recall.
- Dedicated PEEP score and `GET /api/v1/peep` endpoint for confidence-gated early-signal ranking by velocity, source breadth, community spread, and novelty.
- History-backed finite FOMO briefing through `GET /api/v1/fomo`, defaulting to the strongest seven trends from the last 24 hours.
- Propagation timing deltas that show elapsed time from the first observed source rather than only absolute timestamps.
- Production API smoke assertions for health, stream status, and trends, executed after deploy and from a standalone scheduled/manual workflow.
- Optional read-only `GET /api/v1/integrations/notes` publishing bridge protected by `TRENDINARY_INTEGRATION_TOKEN`, exposing only saved/consumed notes explicitly marked worth sharing.

### Changed

- Trendinary Score advances to model version `0.3`, reducing raw-attention weight and increasing velocity plus independent-source/community breadth so unexpected acceleration matters more than fame.
- `/peep` now consumes the live PEEP API instead of filtering the main leaderboard client-side.
- `/fomo` now consumes persisted trend history instead of static prototype data.
- Trend detail propagation presents source order with observed elapsed-time labels such as `origin`, `+14m`, and `+1h 12m`.
- `GET /api/v1/admin/notes?worth_sharing=true` now honors the requested worth-sharing filter instead of returning every saved/consumed item with a note.

## [0.2.5] - 2026-09-01

### Fixed

- Production Cloudflare deployment no longer creates a legacy Page Rule for `www.trendinary.com`; the apex `trendinary.com` Worker/DNS deployment now avoids the Page Rules permission entirely. A modern `www` redirect can be added separately.

## [0.2.4] - 2026-09-01

### Fixed

- Production SST deployment now prints full provider/deployment logs so Cloudflare failures expose their actionable root cause in GitHub Actions instead of only the generic unexpected-error message.

## [0.2.3] - 2026-09-01

### Fixed

- Production Cloudflare deployment now uses Cloudflare provider `6.15.0`, satisfying the minimum provider version required by SST 4.17.1.

## [0.2.2] - 2026-09-01

### Fixed

- Production Fly deploy now invokes `flyctl deploy`, matching the binary installed by the GitHub Actions Fly setup step.

## [0.2.1] - 2026-09-01

### Fixed

- Production deploy preflight now recognizes staged Fly secrets correctly before the first Machine exists.

## [0.2.0] - 2026-09-01

### Added

- Private authenticated editorial workspace under `/admin` with inbox, queue, notes, sources, trash, and newsletter issue assembly.
- TechURLs private discovery adapter with conservative URL normalization/deduplication, ingestion-run accounting, enrichment, deterministic editorial scoring, and optional raw HTML archival.
- RSS/Atom, Wikipedia pageview, GDELT DOC, and optional YouTube public discovery adapters.
- Optional NewsData discovery with durable database-backed daily quota pacing, pagination, replay persistence, and corroboration-only trend behavior.
- Detection Quality v2 entity-aware second-stage clustering, source/author flood controls, repeated-text suppression, stronger candidate gating, historical calibration, and replay evaluation.
- Turso/libSQL production persistence for trend history, stable identities, Jetstream cursors, quota state, and private editorial data while retaining local SQLite for development/tests.
- CI branch-flow guard enforcing normal `feature/* -> dev -> main` release direction.

### Changed

- Fly.io is now a stateless origin; the production database no longer depends on a Fly volume or `/data/trendinary.db`.
- Production startup fails closed unless `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` are configured.
- Production database recovery uses Turso point-in-time recovery/export tooling; the former Fly-volume-to-R2 SQLite backup workflow was removed. R2 remains the raw/provenance archive.
- Public discovery no longer depends on direct Reddit API access. Reddit-adjacent material may arrive indirectly through TechURLs/original publisher links; no Reddit HTML-scraping bypass is added.
- Default source-universe calibration expands for the broader v0.2 discovery set.

## [0.1.0] - 2026-09-01

### Added

- Go 1.27 API server under `/api/v1/*` with the Solid 2/Vite application embedded and served from `/`.
- Fly.io production origin plus SST-managed Cloudflare Worker, DNS, KV, and R2 infrastructure.
- SQLite history, versioned Trendinary Score, lifecycle detection, historical baselines, and momentum history.
- Hacker News and Bluesky/AT Protocol signal ingestion, including Jetstream v2 continuous discovery and durable cursor replay.
- Candidate gating, bounded high-volume stream processing, and source-independent normalized signals.
- GitHub emerging-repository discovery and OAuth-only Reddit discovery.
- Stable trend entities, aliases, canonical terms, and persistent public slugs.
- Selective Bluesky engagement hydration and cached DID/handle/profile resolution.
- Durable propagation/origin tracking across source networks.
- Evidence-backed Source Lens / Perspective Mix metadata that keeps political leaning separate from reliability and truth claims.
- Grounded deterministic WTF, LORE, evidence, and Ask Trendinary responses.
- Live trend intelligence UI with score history, evidence, propagation, source perspective, top voices, and Ask Trendinary.
- Stream/scanner observability through `/api/v1/health/streams`.
- CI verification for TypeScript, Go tests, Vite/server builds, module cleanliness, and the Fly-target Docker image.
