# Trendinary

**The live dictionary of the internet.**

Trendinary watches public internet signals, detects unusual acceleration, clusters related observations into stable trend entities, explains why they are moving, and tracks how attention propagates across sources.

It also contains a private editorial system under `/admin` for deciding what is worth reading/watching/listening to and assembling saved material into newsletter drafts.

## Stack

- Go 1.27 HTTP/API server
- Turso/libSQL as the production durable database
- local SQLite (`modernc.org/sqlite`) for development and tests
- SolidJS 2 + Solid Router
- StyleX
- Vite 8 + TypeScript
- embedded SPA via Go `embed`
- stateless Fly.io origin
- Cloudflare Worker/DNS/KV/R2 managed through SST

## Branch and release flow

```text
feature/* -> dev -> main -> vX.Y.Z -> production
```

Feature work targets `dev`. Normal releases are `dev -> main` pull requests. `main` is release-only except documented hotfixes.

## Public product

- `/` — NOW live scoreboard
- `/peep` — early/accelerating signals
- `/fomo` — catch-up product surface
- `/following` — personal-radar product surface
- `/trend/:slug` — stable trend intelligence page with momentum, propagation, perspective, evidence, WTF/LORE, and Ask

Current discovery sources include Hacker News, ATProto Jetstream/Bluesky, GitHub, configurable RSS/Atom, Wikipedia pageviews, GDELT DOC, optional YouTube, and optional quota-aware NewsData.

Direct Reddit API ingestion is not an active dependency.

## Private editorial admin

Everything under `/admin/*` and `/api/v1/admin/*` is private and enforced by the Go server.

Initial local/admin auth:

```bash
export TRENDINARY_ADMIN_PASSWORD='use-a-long-private-password'
```

Important routes:

- `/admin/inbox`
- `/admin/queue`
- `/admin/notes`
- `/admin/issues`
- `/admin/sources`
- `/admin/trash`

TechURLs is the first private editorial discovery adapter.

Manual ingestion:

```bash
go run ./cmd/trendinary ingest techurls
```

A local SQLite database can still produce a consistent snapshot:

```bash
go run ./cmd/trendinary backup ./trendinary-backup.db
```

That command is intentionally unavailable when the process is connected to Turso; production recovery uses Turso point-in-time recovery/export tooling.

Print current historical calibration:

```bash
go run ./cmd/trendinary calibration
```

## News discovery configuration

GDELT is enabled by default unless explicitly disabled:

```bash
export TRENDINARY_GDELT_DISABLED=1
export TRENDINARY_GDELT_QUERY='(technology OR "artificial intelligence" OR cybersecurity OR startup OR science OR gaming)'
export TRENDINARY_GDELT_LIMIT=250
```

NewsData activates only when a key is present. Its daily calls are durably paced in the shared SQL database rather than burned immediately:

```bash
export TRENDINARY_NEWSDATA_API_KEY='...'
export TRENDINARY_NEWSDATA_DAILY_CALLS=200
export TRENDINARY_NEWSDATA_CATEGORY='technology'
```

RSS/Atom feeds:

```bash
export TRENDINARY_RSS_FEEDS='https://example.com/feed.xml,https://another.example/rss'
```

Optional YouTube discovery:

```bash
export TRENDINARY_YOUTUBE_API_KEY='...'
```

See `docs/news-discovery.md` for source semantics and quota behavior.

## Local development

Install dependencies:

```bash
npm install
```

Run the Go API with local SQLite:

```bash
npm run dev:api
```

Run Vite separately:

```bash
npm run dev
```

Vite proxies `/api` to the local Go server.

To avoid connecting to ATProto during local UI work:

```bash
TRENDINARY_JETSTREAM_DISABLED=1 npm run dev:api
```

You can also point local development at Turso by setting `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`.

## Verification

```bash
npm run verify
```

CI additionally verifies the Go module lock files and builds the production Docker image.

## Production persistence

Fly is stateless. Production requires:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

`fly.toml` sets `TRENDINARY_REQUIRE_TURSO=1`, so production startup fails closed if Turso is not configured. No Fly volume is required.

Turso stores public trend history, stable identities, Jetstream cursors, quota state, and the private editorial database. R2 remains the archive target for raw source provenance/replay payloads rather than the primary database.

See `docs/turso.md` and `docs/architecture.md`.

## Documentation

- `docs/product-vision.md`
- `docs/product-language.md`
- `docs/architecture.md`
- `docs/attention-platform.md`
- `docs/trend-score.md`
- `docs/news-discovery.md`
- `docs/editorial-pipeline.md`
- `docs/sources/techurls.md`
- `docs/turso.md`
- `docs/release-process.md`
