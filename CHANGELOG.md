# Changelog

All notable Trendinary changes are tracked here. Releases use Semantic Versioning and Git tags prefixed with `v`.

## [Unreleased]

### Added

- Future work merged into `dev` accumulates here until the next release PR.

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
