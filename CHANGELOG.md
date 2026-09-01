# Changelog

All notable Trendinary changes are tracked here. Releases use Semantic Versioning and Git tags prefixed with `v`.

## [Unreleased]

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
